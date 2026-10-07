package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/NotRllyRn/life-prep/internal/core"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
)

type fakeMailbox struct {
	days      []int
	histories int
	err       error
}

func (f *fakeMailbox) Backfill(_ context.Context, days int) ([]googlemail.Message, string, error) {
	f.days = append(f.days, days)
	return nil, "42", nil
}
func (f *fakeMailbox) History(_ context.Context, _ string) ([]googlemail.Message, string, error) {
	f.histories++
	return nil, "43", f.err
}
func TestSyncPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, cursor                       string
		force, newOnly, expired, wantError bool
		days                               int
		history                            int
	}{
		{name: "new missing", newOnly: true, wantError: true},
		{name: "new forced", cursor: "1", force: true, newOnly: true, wantError: true},
		{name: "new expired", cursor: "1", newOnly: true, expired: true, wantError: true, history: 1},
		{name: "new valid", cursor: "1", newOnly: true, history: 1},
		{name: "legacy missing", days: 21},
		{name: "legacy forced", cursor: "1", force: true, days: 21},
		{name: "legacy expired", cursor: "1", expired: true, days: -1, history: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeMailbox{}
			if tc.expired {
				f.err = &googlemail.HistoryExpiredError{Cursor: "1"}
			}
			_, _, err := syncMessages(context.Background(), f, tc.cursor, tc.force, tc.newOnly)
			if (err != nil) != tc.wantError || f.histories != tc.history {
				t.Fatalf("err=%v history=%d", err, f.histories)
			}
			if tc.days == 0 && len(f.days) != 0 {
				t.Fatal("unexpected backfill")
			}
			if tc.days != 0 && (len(f.days) != 1 || f.days[0] != tc.days) {
				t.Fatalf("days=%v", f.days)
			}
		})
	}
}
func TestFlagsAndBriefing(t *testing.T) {
	c, err := decodeConfig([]byte(`{"Database":"state.db","Artifacts":"artifacts"}`))
	if err != nil || c.NewOnly || c.DailyBriefing {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	c, err = decodeConfig([]byte(`{"Database":"state.db","Artifacts":"artifacts","NewOnly":true,"DailyBriefing":true}`))
	if err != nil || !c.NewOnly || !c.DailyBriefing {
		t.Fatalf("flags: %+v %v", c, err)
	}
	calls := 0
	generate := func(time.Time) error { calls++; return errors.New("sentinel") }
	if dailyBriefing(false, generate, time.Now()) != nil || calls != 0 {
		t.Fatal("disabled briefing called")
	}
	if dailyBriefing(true, generate, time.Now()) == nil || calls != 1 {
		t.Fatal("enabled briefing not called")
	}
}
func TestSeed(t *testing.T) {
	d := t.TempDir()
	s, err := core.Open(filepath.Join(d, "state.db"), filepath.Join(d, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accounts := []Account{{ID: "a"}, {ID: "b"}}
	calls := 0
	profile := func(_ context.Context, a Account) (string, error) {
		calls++
		if a.ID == "b" {
			return "", errors.New("failed")
		}
		return "100", nil
	}
	if seed(context.Background(), s, accounts, profile) == nil {
		t.Fatal("expected failure")
	}
	if c, _ := s.Cursor("a"); c != "" {
		t.Fatal("partial seed")
	}
	profile = func(_ context.Context, a Account) (string, error) {
		calls++
		if a.ID == "b" {
			return "200", nil
		}
		return "100", nil
	}
	if err = seed(context.Background(), s, accounts, profile); err != nil {
		t.Fatal(err)
	}
	for a, want := range map[string]string{"a": "100", "b": "200"} {
		if c, _ := s.Cursor(a); c != want {
			t.Fatalf("%s=%s", a, c)
		}
	}
	events, _ := s.List()
	if len(events) != 0 {
		t.Fatal("seed created events")
	}
	if err = s.Ingest("a", "101", []googlemail.Message{{ID: "old"}}); err != nil {
		t.Fatal(err)
	}
	before := calls
	if seed(context.Background(), s, accounts, profile) == nil || calls != before {
		t.Fatal("seed must reject events before API")
	}
	if s.Seed(map[string]string{"a": "999"}) == nil {
		t.Fatal("store must also reject events")
	}
	if c, _ := s.Cursor("a"); c != "101" {
		t.Fatal("changed baseline")
	}
}
func TestSeedDisabledAndWriterLock(t *testing.T) {
	d := t.TempDir()
	cfg := Config{Database: filepath.Join(d, "state.db"), Artifacts: filepath.Join(d, "artifacts"), Accounts: []Account{{ID: "a"}, {ID: "b"}}}
	b, _ := json.Marshal(cfg)
	path := filepath.Join(d, "config.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"life-prep", "seed", path}
	err := run()
	if err == nil || strings.Contains(err.Error(), "integrations disabled") {
		t.Fatalf("seed must reach credentials when disabled: %v", err)
	}
	lock, err := os.OpenFile(cfg.Database+".lock", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err = run(); err == nil || !strings.Contains(err.Error(), "another mailbox writer") {
		t.Fatalf("lock: %v", err)
	}
}
