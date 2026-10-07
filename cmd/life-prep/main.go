package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NotRllyRn/life-prep/internal/core"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Account struct{ ID, Email, Credentials, Token, Project, Subscription, PubsubCredentials, Topic string }
type Config struct {
	Enabled             bool
	Database, Artifacts string
	Accounts            []Account
	Hermes              struct {
		Enabled        bool
		URL, SecretEnv string
	}
}

func main() {
	syscall.Umask(0077)
	if e := run(); e != nil {
		log.Print(e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: life-prep COMMAND [config.json] [event-id/result]; commands: serve status doctor queue inspect retry pause resume backfill watch receipt demo")
	}
	cmd := os.Args[1]
	path := "config.json"
	if len(os.Args) > 2 {
		path = os.Args[2]
	}
	var cfg Config
	if cmd == "demo" {
		cfg.Database = path + "/state.db"
		cfg.Artifacts = path + "/artifacts"
	} else {
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if cfg, e = decodeConfig(b); e != nil {
			return e
		}
		if cfg.Database == "" || cfg.Artifacts == "" {
			return errors.New("database and artifacts required")
		}
	}
	s, e := core.Open(cfg.Database, cfg.Artifacts)
	if e != nil {
		return e
	}
	defer s.Close()
	if cmd == "serve" || cmd == "backfill" || cmd == "watch" {
		lock, err := os.OpenFile(cfg.Database+".lock", os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return errors.New("another mailbox writer is running")
		}
		if cmd == "serve" {
			if err = s.Recover(); err != nil {
				return err
			}
		}
	}
	printJSON := func(v any) error {
		b, e := json.MarshalIndent(v, "", "  ")
		if e == nil {
			fmt.Println(string(b))
		}
		return e
	}
	arg := func(n int) (string, error) {
		if len(os.Args) <= n {
			return "", errors.New("missing argument")
		}
		return os.Args[n], nil
	}
	switch cmd {
	case "demo":
		if e = s.Ingest("fixture", "123", []googlemail.Message{{ID: "fixture-1", Subject: "Fixture preparation", Body: "Synthetic credential-free local fixture; not live mail."}}); e != nil {
			return e
		}
		if e = s.Materialize(); e != nil {
			return e
		}
		fallthrough
	case "queue", "status":
		v, e := s.List()
		if e != nil {
			return e
		}
		return printJSON(v)
	case "inspect":
		id, e := arg(3)
		if e != nil {
			return e
		}
		v, e := s.Inspect(id)
		if e != nil {
			return e
		}
		return printJSON(v)
	case "retry":
		id, e := arg(3)
		if e != nil {
			return e
		}
		return s.Retry(id)
	case "receipt":
		id, e := arg(3)
		if e != nil {
			return e
		}
		p, e := arg(4)
		if e != nil {
			return e
		}
		return s.Receipt(id, p)
	case "pause":
		return s.SetPaused(true)
	case "resume":
		return s.SetPaused(false)
	case "doctor":
		if !cfg.Enabled {
			return errors.New("integrations disabled: local queue available; activation gates in docs/setup.md")
		}
		if len(cfg.Accounts) != 2 {
			return errors.New("configure exactly two accounts")
		}
		for _, a := range cfg.Accounts {
			for _, p := range []string{a.Credentials, a.Token, a.PubsubCredentials} {
				if _, e := os.Stat(p); e != nil {
					return e
				}
			}
		}
		if cfg.Hermes.Enabled && os.Getenv(cfg.Hermes.SecretEnv) == "" {
			return errors.New("Hermes secret environment variable missing")
		}
		fmt.Println("Local prerequisites present; live permissions and approvals NOT verified")
		return nil
	}
	if cmd != "serve" && cmd != "watch" && cmd != "backfill" {
		return errors.New("unknown command")
	}
	if !cfg.Enabled {
		return errors.New("integrations disabled")
	}
	if len(cfg.Accounts) != 2 {
		return errors.New("configure exactly two accounts")
	}
	if cfg.Hermes.Enabled && (cfg.Hermes.URL == "" || os.Getenv(cfg.Hermes.SecretEnv) == "") {
		return errors.New("Hermes unconfigured")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var wg sync.WaitGroup
	// Stop and join workers before releasing the writer lock or closing SQLite,
	// including partial startup failures on a later account.
	defer func() { cancel(); wg.Wait() }()
	seen := map[string]bool{}
	for _, a := range cfg.Accounts {
		if a.ID == "" || seen[a.ID] {
			return errors.New("account IDs must be nonempty and unique")
		}
		seen[a.ID] = true
	}
	for _, a := range cfg.Accounts {
		c, e := googlemail.New(ctx, a.Credentials, a.Token)
		if e != nil {
			return e
		}
		if cmd == "watch" {
			if _, e = c.Watch(ctx, a.Topic); e != nil {
				return e
			}
			continue
		}
		var mu sync.Mutex
		syncMail := func(force bool) error {
			mu.Lock()
			defer mu.Unlock()
			cursor, e := s.Cursor(a.ID)
			if e != nil {
				return e
			}
			var ms []googlemail.Message
			var next string
			if cursor == "" || force {
				ms, next, e = c.Backfill(ctx, 21)
			} else {
				ms, next, e = c.History(ctx, cursor)
				var expired *googlemail.HistoryExpiredError
				if errors.As(e, &expired) {
					ms, next, e = c.Backfill(ctx, -1)
				}
			}
			if e != nil {
				return e
			}
			return s.Ingest(a.ID, next, ms)
		}
		if e = syncMail(cmd == "backfill"); e != nil {
			return e
		}
		if cmd == "backfill" {
			continue
		}
		if _, e = c.Watch(ctx, a.Topic); e != nil {
			return e
		}
		wg.Add(1)
		go func(a Account) {
			defer wg.Done()
			for ctx.Err() == nil {
				e := googlemail.Receive(ctx, a.Project, a.Subscription, a.PubsubCredentials, func(_ context.Context, b []byte) error {
					var hint struct{ EmailAddress, HistoryID string }
					if e := json.Unmarshal(b, &hint); e != nil {
						return e
					}
					if a.Email == "" || hint.EmailAddress != a.Email {
						return errors.New("notification mailbox mismatch")
					}
					return syncMail(false)
				})
				if e != nil && ctx.Err() == nil {
					log.Print(e)
					select {
					case <-ctx.Done():
					case <-time.After(30 * time.Second):
					}
				}
			}
		}(a)
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(time.Hour)
			defer t.Stop()
			last := time.Now()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if time.Since(last) >= 24*time.Hour {
						if _, e := c.Watch(ctx, a.Topic); e != nil {
							log.Print(e)
						} else {
							last = time.Now()
						}
					}
					if e := syncMail(false); e != nil {
						log.Print(e)
					}
				}
			}
		}()
	}
	if cmd != "serve" {
		return s.Materialize()
	}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if e := s.Briefing(time.Now()); e != nil {
			log.Print(e)
		}
		if e := s.Materialize(); e != nil {
			log.Print(e)
		}
		if cfg.Hermes.Enabled {
			if e := s.Dispatch(ctx, cfg.Hermes.URL, os.Getenv(cfg.Hermes.SecretEnv)); e != nil {
				log.Print(e)
			}
		}
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-tick.C:
		}
	}
}
