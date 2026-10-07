package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type Store struct {
	db  *sql.DB
	dir string
	mu  sync.Mutex
}
type Event struct{ ID, Account, State, Artifact, Error string }

func Open(path, dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS cursors(account TEXT PRIMARY KEY,cursor TEXT NOT NULL); CREATE TABLE IF NOT EXISTS events(id TEXT PRIMARY KEY,account TEXT,state TEXT,body BLOB,artifact TEXT DEFAULT '',error TEXT DEFAULT ''); CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT); `)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, dir: dir}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Recover() error {
	_, e := s.db.Exec(`UPDATE events SET state='uncertain',error='restart during dispatch; reconcile before retry' WHERE state='dispatching'`)
	return e
}
func (s *Store) Cursor(a string) (string, error) {
	var c string
	err := s.db.QueryRow(`SELECT cursor FROM cursors WHERE account=?`, a).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return c, err
}
func eventID(a, id string) string {
	h := sha256.Sum256([]byte(a + "\x00" + id))
	return hex.EncodeToString(h[:])
}
func (s *Store) Ingest(a, c string, ms []googlemail.Message) error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, m := range ms {
		b, e := json.Marshal(m)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(`INSERT OR IGNORE INTO events(id,account,state,body) VALUES(?,?,'pending',?)`, eventID(a, m.ID), a, b); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`INSERT INTO cursors VALUES(?,?) ON CONFLICT(account) DO UPDATE SET cursor=excluded.cursor`, a, c); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) List() ([]Event, error) {
	rows, e := s.db.Query(`SELECT id,account,state,artifact,error FROM events ORDER BY rowid`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.Account, &v.State, &v.Artifact, &v.Error); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Inspect(id string) (Event, error) {
	var v Event
	e := s.db.QueryRow(`SELECT id,account,state,artifact,error FROM events WHERE id=?`, id).Scan(&v.ID, &v.Account, &v.State, &v.Artifact, &v.Error)
	return v, e
}
func (s *Store) SetPaused(p bool) error {
	_, e := s.db.Exec(`INSERT INTO settings VALUES('paused',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, strconv.FormatBool(p))
	return e
}
func (s *Store) Retry(id string) error {
	r, e := s.db.Exec(`UPDATE events SET state='pending',error='' WHERE id=? AND state='failed'`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("only definitively failed events may retry; uncertain/accepted require reconciliation")
	}
	return nil
}
func (s *Store) Receipt(id, result string) error {
	v, err := s.Inspect(id)
	if err != nil {
		return err
	}
	if v.State != "accepted" && v.State != "uncertain" {
		return errors.New("receipt requires accepted or uncertain event")
	}
	b, e := os.ReadFile(result)
	if e != nil {
		return e
	}
	if len(b) == 0 {
		return errors.New("empty completion artifact")
	}
	dest := filepath.Join(s.dir, id+".result.txt")
	if e = os.WriteFile(dest, b, 0600); e != nil {
		return e
	}
	r, e := s.db.Exec(`UPDATE events SET state='completed',error=? WHERE id=? AND state IN ('accepted','uncertain')`, dest, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("receipt requires accepted or uncertain event")
	}
	return nil
}
func (s *Store) Briefing(now time.Time) error {
	loc, e := time.LoadLocation("America/Los_Angeles")
	if e != nil {
		return e
	}
	t := now.In(loc)
	if t.Hour() < 8 {
		return nil
	}
	return s.Ingest("briefing", "", []googlemail.Message{{ID: t.Format("2006-01-02"), Subject: "Daily preparation briefing", Body: "Review approved local context and produce the designated daily briefing."}})
}

// Materialize always writes real queued content, including when dispatch is disabled.
func (s *Store) Materialize() error {
	events, e := s.List()
	if e != nil {
		return e
	}
	for _, v := range events {
		if v.Artifact != "" {
			continue
		}
		var b []byte
		if e = s.db.QueryRow(`SELECT body FROM events WHERE id=?`, v.ID).Scan(&b); e != nil {
			return e
		}
		p := filepath.Join(s.dir, v.ID+".json")
		tmp := p + ".tmp"
		if e = os.WriteFile(tmp, b, 0600); e != nil {
			return e
		}
		if e = os.Rename(tmp, p); e != nil {
			return e
		}
		if _, e = s.db.Exec(`UPDATE events SET artifact=? WHERE id=?`, p, v.ID); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Dispatch(ctx context.Context, url, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if url == "" || secret == "" {
		return errors.New("Hermes integration unconfigured")
	}
	var paused string
	s.db.QueryRow(`SELECT value FROM settings WHERE key='paused'`).Scan(&paused)
	if paused == "true" {
		return nil
	}
	if e := s.Materialize(); e != nil {
		return e
	}
	vs, e := s.List()
	if e != nil {
		return e
	}
	for _, v := range vs {
		if v.State != "pending" {
			continue
		}
		b, _ := json.Marshal(map[string]any{"event_type": "life-prep.context", "event_id": v.ID, "context": map[string]string{"source_ref": v.Artifact}})
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "."))
		mac.Write(b)
		req, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if e != nil {
			return e
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Timestamp", ts)
		req.Header.Set("X-Webhook-Signature-V2", hex.EncodeToString(mac.Sum(nil)))
		req.Header.Set("X-Request-ID", v.ID)
		if _, e = s.db.Exec(`UPDATE events SET state='dispatching' WHERE id=?`, v.ID); e != nil {
			return e
		}
		client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, e := client.Do(req)
		state, detail := "uncertain", "network outcome unknown"
		if e == nil {
			resp.Body.Close()
			detail = fmt.Sprintf("HTTP %d; acceptance is not completion", resp.StatusCode)
			if resp.StatusCode == 202 {
				state = "accepted"
			} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
				state = "failed"
			}
		} else {
			detail = e.Error()
		}
		if _, e = s.db.Exec(`UPDATE events SET state=?,error=? WHERE id=?`, state, detail, v.ID); e != nil {
			return e
		}
	}
	return nil
}
