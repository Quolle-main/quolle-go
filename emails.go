package quolle

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/url"
	"os"
	"path/filepath"
)

// Emails provides email operations. Access it via Client.Emails.
type Emails struct {
	client *Client
}

// Recipients is the value for the To field: one or many addresses. Use To or
// ToMany to construct it. It marshals to a JSON string for a single recipient
// and a JSON array for multiple, matching the API.
type Recipients struct {
	values []string
}

// To builds a single-recipient value.
func To(addr string) Recipients { return Recipients{values: []string{addr}} }

// ToMany builds a multi-recipient value.
func ToMany(addrs ...string) Recipients { return Recipients{values: addrs} }

// MarshalJSON emits a bare string for one recipient, an array for many.
func (r Recipients) MarshalJSON() ([]byte, error) {
	if len(r.values) == 1 {
		return json.Marshal(r.values[0])
	}
	return json.Marshal(r.values)
}

// SendParams are the fields for a single send. Provide either HTML (with
// Subject) or Template (with optional Variables).
type SendParams struct {
	From        string         `json:"from"`
	To          Recipients     `json:"to"`
	Subject     string         `json:"subject,omitempty"`
	HTML        string         `json:"html,omitempty"`
	Text        string         `json:"text,omitempty"`
	Template    string         `json:"template,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	ReplyTo     string         `json:"replyTo,omitempty"`
	ScheduledAt string         `json:"scheduledAt,omitempty"`
	Attachments []Attachment   `json:"attachments,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	// Type is the message stream: TypeTransactional (the default) or
	// TypeMarketing. Marketing mail gets one-click unsubscribe headers and a
	// footer link, and honours unsubscribes; transactional mail never does.
	Type string `json:"type,omitempty"`
}

// Message stream values for SendParams.Type.
const (
	TypeTransactional = "transactional"
	TypeMarketing     = "marketing"
)

// Attachment is a file attached to an email. Provide either Content (the raw
// file bytes, base64-encoded) or Path (a URL Quolle fetches at send time). Set
// ContentID to embed the file inline via <img src="cid:the-id">. Up to 20
// attachments, 10 MB total.
type Attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content,omitempty"`
	Path        string `json:"path,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	ContentID   string `json:"contentId,omitempty"`
}

// FileAttachment reads a file from disk and returns an Attachment with its
// base64-encoded content and a content type guessed from the extension.
func FileAttachment(path string) (Attachment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, err
	}
	name := filepath.Base(path)
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	return Attachment{
		Filename:    name,
		Content:     base64.StdEncoding.EncodeToString(data),
		ContentType: ctype,
	}, nil
}

// SendResult is returned by Send.
type SendResult struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Status      string `json:"status,omitempty"`
	ScheduledAt string `json:"scheduledAt,omitempty"`
}

// BatchResult is returned by SendBatch.
type BatchResult struct {
	Queued int      `json:"queued"`
	IDs    []string `json:"ids"`
}

// Email is a retrieved email record.
type Email struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	Provider    string `json:"provider"`
	MessageID   string `json:"messageId"`
	OpensCount  int    `json:"opensCount"`
	ClicksCount int    `json:"clicksCount"`
	SentAt      string `json:"sentAt"`
	DeliveredAt string `json:"deliveredAt"`
	BouncedAt   string `json:"bouncedAt"`
	CreatedAt   string `json:"createdAt"`
}

// CancelResult is returned by Cancel.
type CancelResult struct {
	Message string `json:"message"`
}

// SendOption customizes a single Send/SendBatch call.
type SendOption func(map[string]string)

// WithIdempotencyKey makes a send safe to retry — the same key returns the
// original result instead of sending twice.
func WithIdempotencyKey(key string) SendOption {
	return func(h map[string]string) { h["Idempotency-Key"] = key }
}

func headersFrom(opts []SendOption) map[string]string {
	h := map[string]string{}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Send queues an email for immediate or scheduled delivery.
func (e *Emails) Send(ctx context.Context, params SendParams, opts ...SendOption) (*SendResult, error) {
	var out SendResult
	if err := e.client.do(ctx, "POST", "/v1/emails/send", params, headersFrom(opts), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendBatch sends up to 100 emails in one all-or-nothing request.
func (e *Emails) SendBatch(ctx context.Context, emails []SendParams, opts ...SendOption) (*BatchResult, error) {
	body := map[string]any{"emails": emails}
	var out BatchResult
	if err := e.client.do(ctx, "POST", "/v1/emails/batch", body, headersFrom(opts), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get retrieves a single email by ID.
func (e *Emails) Get(ctx context.Context, id string) (*Email, error) {
	var wrapper struct {
		Email Email `json:"email"`
	}
	if err := e.client.do(ctx, "GET", "/v1/emails/"+url.PathEscape(id), nil, nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Email, nil
}

// Cancel cancels a scheduled email that has not yet been sent.
func (e *Emails) Cancel(ctx context.Context, id string) (*CancelResult, error) {
	var out CancelResult
	if err := e.client.do(ctx, "DELETE", "/v1/emails/"+url.PathEscape(id)+"/cancel", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
