package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	d := t.TempDir()
	s, e := Open(filepath.Join(d, "s.db"), filepath.Join(d, "artifacts"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ms := []googlemail.Message{{ID: "a", Body: "untrusted fixture"}}
	for i := 0; i < 2; i++ {
		if e = s.Ingest("one", "123", ms); e != nil {
			t.Fatal(e)
		}
	}
	vs, _ := s.List()
	if len(vs) != 1 {
		t.Fatal(vs)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		m := hmac.New(sha256.New, []byte("fixture-secret"))
		m.Write([]byte(r.Header.Get("X-Webhook-Timestamp") + "."))
		m.Write(b)
		if r.Header.Get("X-Webhook-Signature-V2") != hex.EncodeToString(m.Sum(nil)) {
			t.Error("signature")
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()
	if e = s.Dispatch(context.Background(), srv.URL, "fixture-secret"); e != nil {
		t.Fatal(e)
	}
	v, _ := s.Inspect(vs[0].ID)
	if v.State != "accepted" || v.Artifact == "" {
		t.Fatal(v)
	}
	if s.Retry(v.ID) == nil {
		t.Fatal("unsafe retry")
	}
	c, _ := s.Cursor("one")
	if c != "123" {
		t.Fatal(c)
	}
}
func TestCrashAndDST(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "s.db")
	s, _ := Open(p, d)
	s.Ingest("a", "1", []googlemail.Message{{ID: "1"}})
	s.db.Exec(`UPDATE events SET state='dispatching'`)
	s.Close()
	s, e := Open(p, d)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.Recover()
	v, _ := s.List()
	if v[0].State != "uncertain" {
		t.Fatal(v)
	}
	loc, _ := time.LoadLocation("America/Los_Angeles")
	for _, date := range []string{"2026-03-08", "2026-11-01"} {
		tm, _ := time.ParseInLocation("2006-01-02 15:04", date+" 07:59", loc)
		s.Briefing(tm)
		s.Briefing(tm.Add(time.Minute))
		s.Briefing(tm.Add(time.Hour))
	}
	v, _ = s.List()
	if len(v) != 3 {
		t.Fatal(v)
	}
}
