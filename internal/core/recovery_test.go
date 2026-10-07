package core

import (
	"context"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackPauseReceiptAndAmbiguity(t *testing.T) {
	d := t.TempDir()
	s, e := Open(filepath.Join(d, "db"), filepath.Join(d, "art"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.db.Exec(`CREATE TRIGGER reject_cursor BEFORE INSERT ON cursors BEGIN SELECT RAISE(ABORT,'fixture failure'); END`)
	if s.Ingest("a", "1", []googlemail.Message{{ID: "m"}}) == nil {
		t.Fatal("expected failed transaction")
	}
	vs, _ := s.List()
	if len(vs) != 0 {
		t.Fatal("queue escaped rollback")
	}
	s.db.Exec(`DROP TRIGGER reject_cursor`)
	if e = s.Ingest("a", "1", []googlemail.Message{{ID: "m"}}); e != nil {
		t.Fatal(e)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer srv.Close()
	s.SetPaused(true)
	s.Dispatch(context.Background(), srv.URL, "fixture")
	if calls != 0 {
		t.Fatal("paused dispatch")
	}
	s.SetPaused(false)
	s.Dispatch(context.Background(), srv.URL, "fixture")
	s.Dispatch(context.Background(), srv.URL, "fixture")
	if calls != 1 {
		t.Fatal("ambiguous dispatch repeated")
	}
	vs, _ = s.List()
	if vs[0].State != "uncertain" {
		t.Fatal(vs)
	}
	result := filepath.Join(d, "result")
	os.WriteFile(result, []byte("Operator verified fixture outcome."), 0600)
	if e = s.Receipt(vs[0].ID, result); e != nil {
		t.Fatal(e)
	}
	v, _ := s.Inspect(vs[0].ID)
	if v.State != "completed" {
		t.Fatal(v)
	}
	if s.Receipt("../../escape", result) == nil {
		t.Fatal("invalid event receipt")
	}
}
