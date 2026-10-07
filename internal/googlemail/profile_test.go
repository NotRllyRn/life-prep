package googlemail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestCurrentHistoryProfileOnly(t *testing.T) {
	for _, tc := range []struct {
		body, want string
		fail       bool
	}{
		{`{"historyId":"987654321"}`, "987654321", false},
		{`{"historyId":"0"}`, "", true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/gmail/v1/users/me/profile" {
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			svc, err := gmail.NewService(context.Background(), option.WithEndpoint(srv.URL+"/"), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			got, err := NewService(svc).CurrentHistory(context.Background())
			if got != tc.want || (err != nil) != tc.fail || calls != 1 {
				t.Fatalf("got=%s err=%v calls=%d", got, err, calls)
			}
		})
	}
}
