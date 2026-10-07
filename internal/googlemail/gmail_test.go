package googlemail

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	service, err := gmail.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	return NewService(service)
}
func TestBackfillPaginationAndBody(t *testing.T) {
	pages := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/profile"):
			fmt.Fprint(w, `{"historyId":"100"}`)
		case strings.HasSuffix(r.URL.Path, "/messages"):
			pages++
			if !strings.HasPrefix(r.URL.Query().Get("q"), "after:") || !strings.HasSuffix(r.URL.Query().Get("q"), " -in:sent -in:drafts") {
				t.Error("missing time or received-scope query")
			}
			if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{"messages":[{"id":"a"}],"nextPageToken":"next"}`)
			} else {
				fmt.Fprint(w, `{"messages":[{"id":"a"},{"id":"b"}]}`)
			}
		default:
			if r.URL.Query().Get("format") != "full" {
				t.Error("not full format")
			}
			fmt.Fprint(w, `{"id":"x","threadId":"thread","payload":{"mimeType":"multipart/alternative","headers":[{"name":"subject","value":"Subject"},{"name":"From","value":"sender"}],"parts":[{"mimeType":"text/html","body":{"data":"PGI-aGk8L2I-"}},{"mimeType":"text/plain","body":{"data":"aGk="}}]}}`)
		}
	})
	messages, cursor, err := c.Backfill(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(messages) != 2 || cursor != "100" || messages[0].Body != "hi" || messages[0].Subject != "Subject" || messages[0].From != "sender" {
		t.Fatalf("unexpected %v %s pages=%d", messages, cursor, pages)
	}
}
func TestHistoryPagination(t *testing.T) {
	pages := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/history") {
			pages++
			if r.URL.Query().Get("startHistoryId") != "100" {
				t.Error("wrong cursor")
			}
			if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{"historyId":"200","history":[{"messagesAdded":[{"message":{"id":"a"}}]}],"nextPageToken":"next"}`)
			} else {
				fmt.Fprint(w, `{"historyId":"201","history":[{"messagesAdded":[{"message":{"id":"a"}},{"message":{"id":"b"}}]}]}`)
			}
		} else {
			fmt.Fprint(w, `{"id":"message"}`)
		}
	})
	messages, cursor, err := c.History(context.Background(), "100")
	if err != nil || len(messages) != 2 || cursor != "201" || pages != 2 {
		t.Fatalf("%v %s %v", messages, cursor, err)
	}
}
func TestHistoryExpired(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":404,"message":"expired"}}`, 404)
	})
	_, cursor, err := c.History(context.Background(), "100")
	var expired *HistoryExpiredError
	if !errors.As(err, &expired) || cursor != "" || expired.Cursor != "100" {
		t.Fatalf("%s %v", cursor, err)
	}
}
func TestWatchAndRenewCancellation(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/watch") {
			t.Error("wrong watch request")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"historyId":"123","expiration":"9999999999999"}`)
	})
	cursor, err := c.Watch(context.Background(), "projects/p/topics/t")
	if err != nil || cursor != "123" {
		t.Fatal(cursor, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = c.RenewWatch(ctx, "projects/p/topics/t"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestDeliveryAcknowledgement(t *testing.T) {
	for _, fail := range []bool{false, true} {
		ack, nack := 0, 0
		deliver(context.Background(), []byte("payload"), func(_ context.Context, b []byte) error {
			if string(b) != "payload" {
				t.Error("payload changed")
			}
			if fail {
				return errors.New("retry")
			}
			return nil
		}, func() { ack++ }, func() { nack++ })
		if fail && (ack != 0 || nack != 1) || !fail && (ack != 1 || nack != 0) {
			t.Fatalf("ack=%d nack=%d", ack, nack)
		}
	}
}
