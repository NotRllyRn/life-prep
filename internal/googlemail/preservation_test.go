package googlemail

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const preservedMessage = `{"id":"m","threadId":"t","labelIds":["INBOX","UNREAD","custom"],"internalDate":"1700000000123","payload":{"mimeType":"multipart/mixed","headers":[{"name":"DATE","value":"original date"},{"name":"Subject","value":"subject"},{"name":"From","value":"sender"}],"parts":[{"partId":"0","mimeType":"text/plain","body":{"data":"aGk="}},{"partId":"1","mimeType":"application/pdf","filename":"invoice.pdf","body":{"attachmentId":"pdf","size":456}},{"partId":"2","mimeType":"text/plain","body":{"attachmentId":"external-text","size":123}},{"partId":"3","mimeType":"text/plain","headers":[{"name":"Content-Disposition","value":"attachment; filename=x"}],"body":{"data":"c2VjcmV0","size":6}}]}}`

func TestPreservationAndExplicitReadonlyHelpers(t *testing.T) {
	downloads := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("non-readonly method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/messages/m/attachments/pdf"):
			downloads++
			fmt.Fprint(w, `{"data":"cGRm","size":3}`)
		case strings.HasSuffix(r.URL.Path, "/threads/t"):
			if r.URL.Query().Get("format") != "full" {
				t.Error("thread not full")
			}
			fmt.Fprintf(w, `{"id":"t","messages":[%s,{"id":"sent","labelIds":["SENT"]}]}`, preservedMessage)
		case strings.HasSuffix(r.URL.Path, "/messages/m"):
			fmt.Fprint(w, preservedMessage)
		default:
			t.Errorf("unexpected automatic download: %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	})
	ctx := context.Background()
	m, err := c.Get(ctx, "m")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "m" || m.ThreadID != "t" || m.Date != "original date" || m.InternalDate != 1700000000123 || len(m.LabelIDs) != 3 || m.LabelIDs[2] != "custom" || m.Body != "hi" || m.Subject != "subject" || m.From != "sender" {
		t.Fatalf("metadata lost: %+v", m)
	}
	if len(m.Attachments) != 3 || m.Attachments[0] != (AttachmentRef{ID: "pdf", PartID: "1", Filename: "invoice.pdf", MimeType: "application/pdf", Size: 456}) || m.Attachments[1].ID != "external-text" || m.Attachments[2].PartID != "3" {
		t.Fatalf("refs lost: %+v", m.Attachments)
	}
	thread, err := c.Thread(ctx, "t")
	if err != nil || len(thread) != 2 || thread[0].Body != "hi" || thread[1].LabelIDs[0] != "SENT" {
		t.Fatalf("thread: %+v %v", thread, err)
	}
	if downloads != 0 {
		t.Fatal("automatic attachment download")
	}
	data, err := c.Attachment(ctx, "m", "pdf")
	if err != nil || string(data) != "pdf" || downloads != 1 {
		t.Fatalf("attachment: %q %v downloads=%d", data, err, downloads)
	}
}

func TestReceivedSyncScope(t *testing.T) {
	for _, mode := range []string{"backfill", "history"} {
		t.Run(mode, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/profile"):
					fmt.Fprint(w, `{"historyId":"100"}`)
				case strings.HasSuffix(r.URL.Path, "/messages"):
					q := r.URL.Query().Get("q")
					if q != "-in:sent -in:drafts" {
						t.Errorf("all-time query: %q", q)
					}
					if len(r.URL.Query()["labelIds"]) != 0 {
						t.Error("must not require INBOX")
					}
					fmt.Fprint(w, `{"messages":[{"id":"archived"},{"id":"sent"},{"id":"draft"},{"id":"spam"},{"id":"trash"},{"id":"gone"}]}`)
				case strings.HasSuffix(r.URL.Path, "/history"):
					if r.URL.Query().Get("historyTypes") != "messageAdded" {
						t.Error("wrong history type")
					}
					fmt.Fprint(w, `{"historyId":"200","history":[{"messagesAdded":[{"message":{"id":"archived"}},{"message":{"id":"sent"}},{"message":{"id":"draft"}},{"message":{"id":"spam"}},{"message":{"id":"trash"}},{"message":{"id":"gone"}}]}]}`)
				case strings.HasSuffix(r.URL.Path, "/sent"):
					fmt.Fprint(w, `{"id":"sent","labelIds":["SENT","INBOX"]}`)
				case strings.HasSuffix(r.URL.Path, "/draft"):
					fmt.Fprint(w, `{"id":"draft","labelIds":["DRAFT"]}`)
				case strings.HasSuffix(r.URL.Path, "/spam"):
					fmt.Fprint(w, `{"id":"spam","labelIds":["SPAM"]}`)
				case strings.HasSuffix(r.URL.Path, "/trash"):
					fmt.Fprint(w, `{"id":"trash","labelIds":["TRASH"]}`)
				case strings.HasSuffix(r.URL.Path, "/gone"):
					http.Error(w, `{"error":{"code":404}}`, 404)
				default:
					fmt.Fprint(w, `{"id":"archived","labelIds":["custom"]}`)
				}
			})
			var messages []Message
			var cursor string
			var err error
			wantCursor := "100"
			if mode == "backfill" {
				messages, cursor, err = c.Backfill(context.Background(), -1)
			} else {
				messages, cursor, err = c.History(context.Background(), "100")
				wantCursor = "200"
			}
			if err != nil || len(messages) != 1 || messages[0].ID != "archived" || cursor != wantCursor {
				t.Fatalf("received scope: %+v %q %v", messages, cursor, err)
			}
		})
	}
}

func TestSyncFailureDoesNotExposePartialResultOrCursor(t *testing.T) {
	for _, mode := range []string{"backfill", "history"} {
		for _, failure := range []string{"page", "message", "decode"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/profile"):
						fmt.Fprint(w, `{"historyId":"100"}`)
					case strings.HasSuffix(r.URL.Path, "/messages"), strings.HasSuffix(r.URL.Path, "/history"):
						if r.URL.Query().Get("pageToken") != "" {
							if failure == "page" {
								http.Error(w, `{"error":{"code":500}}`, 500)
								return
							}
							if mode == "backfill" {
								fmt.Fprint(w, `{"messages":[{"id":"bad"}]}`)
							} else {
								fmt.Fprint(w, `{"historyId":"300","history":[{"messagesAdded":[{"message":{"id":"bad"}}]}]}`)
							}
						} else if mode == "backfill" {
							fmt.Fprint(w, `{"messages":[{"id":"ok"}],"nextPageToken":"next"}`)
						} else {
							fmt.Fprint(w, `{"historyId":"200","history":[{"messagesAdded":[{"message":{"id":"ok"}}]}],"nextPageToken":"next"}`)
						}
					case strings.HasSuffix(r.URL.Path, "/bad"):
						if failure == "message" {
							http.Error(w, `{"error":{"code":500}}`, 500)
						} else {
							fmt.Fprint(w, `{"id":"bad","payload":{"mimeType":"text/plain","body":{"data":"!invalid"}}}`)
						}
					default:
						fmt.Fprint(w, `{"id":"ok"}`)
					}
				})
				var messages []Message
				var cursor string
				var err error
				if mode == "backfill" {
					messages, cursor, err = c.Backfill(context.Background(), 1)
				} else {
					messages, cursor, err = c.History(context.Background(), "100")
				}
				if err == nil || messages != nil || cursor != "" {
					t.Fatalf("partial result escaped: %+v %q %v", messages, cursor, err)
				}
			})
		}
	}
}

func TestReadonlyHelpersPropagateErrors(t *testing.T) {
	for _, status := range []int{404, 500} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, fmt.Sprintf(`{"error":{"code":%d}}`, status), status)
		})
		if _, err := c.Thread(context.Background(), "t"); err == nil {
			t.Fatal("missing thread error")
		}
		if _, err := c.Attachment(context.Background(), "m", "a"); err == nil {
			t.Fatal("missing attachment error")
		}
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":"!invalid"}`)
	})
	if _, err := c.Attachment(context.Background(), "m", "a"); err == nil {
		t.Fatal("missing attachment decode error")
	}
}
