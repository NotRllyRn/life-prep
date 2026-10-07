package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/NotRllyRn/life-prep/internal/googlemail"
)

func TestArtifactProvenance(t *testing.T) {
	s := reviewStore(t)
	if err := s.Materialize(); err != nil {
		t.Fatal(err)
	}
	events, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(events[0].Artifact)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		EventID, Account string
		Message          googlemail.Message
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.EventID != events[0].ID || envelope.Account != "fixture" || envelope.Message.ID != "1" {
		t.Fatalf("lost provenance: %+v", envelope)
	}
}

func TestPauseDuringBatch(t *testing.T) {
	s := reviewStore(t)
	if err := s.Ingest("fixture", "2", []googlemail.Message{{ID: "2"}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := s.SetPaused(true); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	if err := s.Dispatch(context.Background(), server.URL, "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("pause did not stop batch: %d requests", calls)
	}
	events, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if events[0].State != "accepted" || events[1].State != "pending" {
		t.Fatal(events)
	}
}
