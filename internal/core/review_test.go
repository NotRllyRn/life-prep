package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/NotRllyRn/life-prep/internal/googlemail"
)

func reviewStore(t *testing.T) *Store {
	t.Helper()
	d := t.TempDir()
	s, err := Open(filepath.Join(d, "state.db"), filepath.Join(d, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Ingest("fixture", "1", []googlemail.Message{{ID: "1"}}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPauseUnconfiguredAndQueryFailure(t *testing.T) {
	s := reviewStore(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(202) }))
	defer srv.Close()
	if s.Dispatch(context.Background(), "", "") == nil {
		t.Fatal("unconfigured accepted")
	}
	if err := s.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Dispatch(context.Background(), srv.URL, "secret"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("paused dispatch sent")
	}
	if _, err := s.db.Exec(`DROP TABLE settings`); err != nil {
		t.Fatal(err)
	}
	if s.Dispatch(context.Background(), srv.URL, "secret") == nil {
		t.Fatal("pause query error ignored")
	}
	if calls != 0 {
		t.Fatal("sent despite query failure")
	}
}

func TestUnknownReceiptWritesNothing(t *testing.T) {
	s := reviewStore(t)
	if s.Receipt(strings.Repeat("0", 64), "missing") == nil {
		t.Fatal("unknown event accepted")
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("unknown receipt wrote files", entries)
	}
}

func TestNetworkFailureIsUncertain(t *testing.T) {
	s := reviewStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Dispatch(ctx, "http://127.0.0.1:1", "secret"); err != nil {
		t.Fatal(err)
	}
	vs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if vs[0].State != "uncertain" {
		t.Fatal(vs)
	}
	if s.Retry(vs[0].ID) == nil {
		t.Fatal("uncertain replay allowed")
	}
}

func TestConcurrentReceiptsDoNotOverwrite(t *testing.T) {
	s := reviewStore(t)
	vs, _ := s.List()
	id := vs[0].ID
	if _, err := s.db.Exec(`UPDATE events SET state='accepted' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	other, err := Open(filepath.Join(filepath.Dir(s.dir), "state.db"), s.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	paths := []string{filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")}
	for i, p := range paths {
		if err := os.WriteFile(p, []byte(p), 0600); err != nil {
			t.Fatal(i, err)
		}
	}
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for i, store := range []*Store{s, other} {
		wg.Add(1)
		go func(st *Store, p string) { defer wg.Done(); out <- st.Receipt(id, p) }(store, paths[i])
	}
	wg.Wait()
	close(out)
	success := 0
	for err := range out {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("receipt successes", success)
	}
	v, err := s.Inspect(id)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "completed" || v.Result == "" || v.Error != "" {
		t.Fatal(v)
	}
	before, err := os.ReadFile(v.Result)
	if err != nil {
		t.Fatal(err)
	}
	if s.Receipt(id, paths[0]) == nil {
		t.Fatal("completed receipt replaced")
	}
	after, err := os.ReadFile(v.Result)
	if err != nil || string(after) != string(before) {
		t.Fatal("result overwritten", err)
	}
}

func TestDispatchOutcomeCannotUndoReceipt(t *testing.T) {
	s := reviewStore(t)
	vs, _ := s.List()
	id := vs[0].ID
	result := filepath.Join(t.TempDir(), "result")
	if err := os.WriteFile(result, []byte("verified"), 0600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Model reconciliation after an in-flight attempt was marked uncertain.
		if _, err := s.db.Exec(`UPDATE events SET state='uncertain' WHERE id=?`, id); err != nil {
			t.Error(err)
		}
		if err := s.Receipt(id, result); err != nil {
			t.Error(err)
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()
	if err := s.Dispatch(context.Background(), srv.URL, "secret"); err != nil {
		t.Fatal(err)
	}
	v, err := s.Inspect(id)
	if err != nil || v.State != "completed" {
		t.Fatal(v, err)
	}
}

func TestSchoolContextSurvivesArtifact(t *testing.T) {
	s := reviewStore(t)
	m := googlemail.Message{ID: "school", Date: "original date", InternalDate: 123, LabelIDs: []string{"school-origin"}, Attachments: []googlemail.AttachmentRef{{ID: "attachment", Filename: "assignment.pdf", Size: 42}}}
	if err := s.Ingest("school", "2", []googlemail.Message{m}); err != nil {
		t.Fatal(err)
	}
	if err := s.Materialize(); err != nil {
		t.Fatal(err)
	}
	v, err := s.Inspect(eventID("school", "school"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(v.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	var got googlemail.Message
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Date != m.Date || got.InternalDate != 123 || len(got.LabelIDs) != 1 || got.LabelIDs[0] != "school-origin" || len(got.Attachments) != 1 || got.Attachments[0] != m.Attachments[0] {
		t.Fatal(got)
	}
}

func TestResultMigration(t *testing.T) {
	s := reviewStore(t)
	if _, err := s.db.Exec(`ALTER TABLE events DROP COLUMN result`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE events SET state='completed',error='legacy.result.txt'`); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(s.dir), "state.db")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, s.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Result != "legacy.result.txt" || events[0].Error != "" {
		t.Fatal(events)
	}
}
