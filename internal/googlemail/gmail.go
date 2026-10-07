// Package googlemail provides read-only Gmail synchronization and Pub/Sub delivery.
package googlemail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type Message struct{ ID, ThreadID, Subject, From, Body string }
type Client struct{ service *gmail.Service }

// HistoryExpiredError means a full Backfill is required; do not advance the cursor.
type HistoryExpiredError struct {
	Cursor string
	Err    error
}

func (e *HistoryExpiredError) Error() string { return "gmail history cursor expired: " + e.Cursor }
func (e *HistoryExpiredError) Unwrap() error { return e.Err }

// New uses an existing OAuth client credentials JSON and token JSON. It never
// launches an interactive authorization flow. The token must grant Gmail readonly.
func New(ctx context.Context, credentialsFile, tokenFile string) (*Client, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read OAuth credentials: %w", err)
	}
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, err
	}
	b, err = os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read OAuth token: %w", err)
	}
	var token oauth2.Token
	if err = json.Unmarshal(b, &token); err != nil {
		return nil, fmt.Errorf("decode OAuth token: %w", err)
	}
	if token.AccessToken == "" && token.RefreshToken == "" {
		return nil, errors.New("OAuth token has no access or refresh token")
	}
	service, err := gmail.NewService(ctx, option.WithHTTPClient(config.Client(ctx, &token)))
	if err != nil {
		return nil, err
	}
	return NewService(service), nil
}

// NewService injects an official SDK service, including an httptest endpoint.
func NewService(service *gmail.Service) *Client { return &Client{service: service} }

// Backfill defaults to 21 days. Capture the cursor before listing so changes
// during pagination remain available to the following History call.
func (c *Client) Backfill(ctx context.Context, days int) ([]Message, string, error) {
	if days == 0 {
		days = 21
	}
	profile, err := c.service.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return nil, "", err
	}
	query := ""
	if days > 0 {
		query = "after:" + strconv.FormatInt(time.Now().AddDate(0, 0, -days).Unix(), 10)
	}
	var out []Message
	seen := map[string]bool{}
	page := ""
	for {
		response, err := c.service.Users.Messages.List("me").Q(query).MaxResults(500).PageToken(page).Context(ctx).Do()
		if err != nil {
			return nil, "", err
		}
		for _, ref := range response.Messages {
			if seen[ref.Id] {
				continue
			}
			seen[ref.Id] = true
			message, err := c.Get(ctx, ref.Id)
			if isNotFound(err) {
				continue
			}
			if err != nil {
				return nil, "", err
			}
			out = append(out, message)
		}
		page = response.NextPageToken
		if page == "" {
			break
		}
	}
	return out, strconv.FormatUint(profile.HistoryId, 10), nil
}

// History returns newly added messages, not deletion or label-change events.
func (c *Client) History(ctx context.Context, cursor string) ([]Message, string, error) {
	id, err := strconv.ParseUint(cursor, 10, 64)
	if err != nil || id == 0 {
		return nil, "", fmt.Errorf("invalid Gmail history cursor %q", cursor)
	}
	var out []Message
	seen := map[string]bool{}
	page := ""
	latest := id
	for {
		response, err := c.service.Users.History.List("me").StartHistoryId(id).HistoryTypes("messageAdded").MaxResults(500).PageToken(page).Context(ctx).Do()
		if isNotFound(err) {
			return nil, "", &HistoryExpiredError{Cursor: cursor, Err: err}
		}
		if err != nil {
			return nil, "", err
		}
		if response.HistoryId > latest {
			latest = response.HistoryId
		}
		for _, record := range response.History {
			for _, added := range record.MessagesAdded {
				if added.Message == nil || seen[added.Message.Id] {
					continue
				}
				seen[added.Message.Id] = true
				message, err := c.Get(ctx, added.Message.Id)
				if isNotFound(err) {
					continue
				}
				if err != nil {
					return nil, "", err
				}
				out = append(out, message)
			}
		}
		page = response.NextPageToken
		if page == "" {
			break
		}
	}
	return out, strconv.FormatUint(latest, 10), nil
}
func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 404
}

// Get fetches headers and recursively decodes inline MIME bodies. Plain text is
// preferred; HTML is returned unchanged when no plain-text body exists.
func (c *Client) Get(ctx context.Context, id string) (Message, error) {
	raw, err := c.service.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
	if err != nil {
		return Message{}, err
	}
	out := Message{ID: raw.Id, ThreadID: raw.ThreadId}
	if raw.Payload == nil {
		return out, nil
	}
	for _, h := range raw.Payload.Headers {
		switch {
		case strings.EqualFold(h.Name, "Subject"):
			out.Subject = h.Value
		case strings.EqualFold(h.Name, "From"):
			out.From = h.Value
		}
	}
	plain, html, err := c.body(ctx, id, raw.Payload)
	if err != nil {
		return Message{}, err
	}
	out.Body = strings.Join(plain, "\n")
	if len(plain) == 0 {
		out.Body = strings.Join(html, "\n")
	}
	return out, nil
}
func (c *Client) body(ctx context.Context, id string, p *gmail.MessagePart) (plain, html []string, err error) {
	if p == nil {
		return
	}
	if p.MimeType == "text/plain" || p.MimeType == "text/html" {
		if p.Filename != "" {
			return
		}
		if p.Body != nil {
			data := p.Body.Data
			if data == "" && p.Body.AttachmentId != "" {
				var b *gmail.MessagePartBody
				b, err = c.service.Users.Messages.Attachments.Get("me", id, p.Body.AttachmentId).Context(ctx).Do()
				if err != nil {
					return
				}
				data = b.Data
			}
			var decoded []byte
			decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(data, "="))
			if err != nil {
				return
			}
			if p.MimeType == "text/plain" {
				plain = append(plain, string(decoded))
			} else {
				html = append(html, string(decoded))
			}
		}
	}
	for _, part := range p.Parts {
		var a, b []string
		a, b, err = c.body(ctx, id, part)
		if err != nil {
			return
		}
		plain = append(plain, a...)
		html = append(html, b...)
	}
	return
}

// Watch starts or renews a watch. Call daily; topic must be a fully qualified
// projects/PROJECT/topics/TOPIC name in the OAuth client's project.
func (c *Client) Watch(ctx context.Context, topic string) (string, error) {
	response, err := c.service.Users.Watch("me", &gmail.WatchRequest{TopicName: topic}).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(response.HistoryId, 10), nil
}

// RenewWatch renews immediately, then daily until cancellation. Renewal cursors
// are deliberately discarded: they must not replace the processed sync cursor.
func (c *Client) RenewWatch(ctx context.Context, topic string) error {
	for {
		if _, err := c.Watch(ctx, topic); err != nil {
			return err
		}
		timer := time.NewTimer(24 * time.Hour)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
