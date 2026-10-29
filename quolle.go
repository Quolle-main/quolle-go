// Package quolle is the official Go client for the Quolle email API.
//
//	client := quolle.New("qle_...")
//	res, err := client.Emails.Send(context.Background(), quolle.SendParams{
//	    From:    "hello@mail.yourdomain.com",
//	    To:      quolle.To("customer@example.com"),
//	    Subject: "Welcome!",
//	    HTML:    "<h1>Thanks for signing up</h1>",
//	})
//	if err != nil { log.Fatal(err) }
//	fmt.Println(res.ID)
package quolle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "1.0.0"

const defaultBaseURL = "https://api.quolle.com"

// Client is the Quolle API client. Create one with New.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int

	// Emails exposes the email operations (Send, SendBatch, Get, Cancel).
	Emails *Emails
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (default https://api.quolle.com).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(url, "/") }
}

// WithHTTPClient supplies a custom *http.Client (e.g. with a proxy or timeout).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithMaxRetries sets how many times to retry transient failures (default 3).
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// noRetryCodes are 429s that retrying can't fix: the limit resets at a fixed
// time (midnight UTC, the next billing period), not in seconds. Returned at once
// so the caller can queue the send instead of blocking in a retry loop.
var noRetryCodes = map[string]bool{
	"daily_quota_exceeded":   true,
	"warmup_limit_exceeded":  true,
	"monthly_quota_exceeded": true,
}

// maxRetryWait: a Retry-After longer than this means "not soon"; return the
// error instead of sleeping through it.
const maxRetryWait = 60 * time.Second

// retryAfterTooLong reports whether a Retry-After header asks for longer than
// maxRetryWait.
func retryAfterTooLong(retryAfter string) bool {
	secs, err := strconv.Atoi(strings.TrimSpace(retryAfter))
	return err == nil && time.Duration(secs)*time.Second > maxRetryWait
}

// sleepBackoff waits before the next retry: Retry-After if provided, else
// exponential backoff (0.5s, 1s, 2s, …) capped at 8s with jitter. Respects ctx.
func sleepBackoff(ctx context.Context, attempt int, retryAfter string) {
	var d time.Duration
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs >= 0 {
			d = time.Duration(secs) * time.Second
		}
	}
	if d == 0 {
		base := time.Duration(500*(1<<attempt)) * time.Millisecond
		if base > 8*time.Second {
			base = 8 * time.Second
		}
		d = base + time.Duration(rand.Intn(250))*time.Millisecond
	}
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// New creates a Client. If apiKey is empty, it falls back to the QUOLLE_API_KEY
// environment variable. It panics only if no key can be found.
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("QUOLLE_API_KEY")
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
	}
	for _, o := range opts {
		o(c)
	}
	c.Emails = &Emails{client: c}
	return c
}

// Error is returned when the API responds with a non-2xx status.
type Error struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Code is the API's stable, machine-readable error code (e.g.
	// "validation_error", "rate_limit_exceeded", "warmup_limit_exceeded").
	// Branch on this rather than on Message. Empty for transport failures.
	Code string
	// Message is the human-readable error (from the API's "error" field).
	Message string
	// Data holds any extra fields returned alongside "error".
	Data map[string]any
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("quolle: [%d %s] %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("quolle: [%d] %s", e.StatusCode, e.Message)
}

// do performs an authenticated request. out may be nil.
func (c *Client) do(ctx context.Context, method, path string, body any, headers map[string]string, out any) error {
	if c.apiKey == "" {
		return &Error{Message: "a Quolle API key is required (pass it to New or set QUOLLE_API_KEY)"}
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("quolle: encoding request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	// Retry safety: a 429 (rate limited — never processed) is always safe to
	// retry. A 5xx/network error is only retried for idempotent methods
	// (GET/DELETE) or when an Idempotency-Key is set, so a POST can't double-send.
	idempotent := method == http.MethodGet || method == http.MethodDelete
	_, hasIdem := headers["Idempotency-Key"]

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var reqReader io.Reader
		if reader != nil {
			// Reset the body for each attempt.
			if br, ok := reader.(*bytes.Reader); ok {
				_, _ = br.Seek(0, io.SeekStart)
			}
			reqReader = reader
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqReader)
		if err != nil {
			return fmt.Errorf("quolle: building request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "quolle-go/"+Version)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &Error{Message: "network error: " + err.Error()}
			if attempt < c.maxRetries && (idempotent || hasIdem) {
				sleepBackoff(ctx, attempt, "")
				continue
			}
			return lastErr
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := parseError(resp.StatusCode, raw)
			lastErr = apiErr
			retryable := (resp.StatusCode == 429 && !noRetryCodes[apiErr.Code]) ||
				((resp.StatusCode >= 500 && resp.StatusCode <= 504) && (idempotent || hasIdem))
			if attempt < c.maxRetries && retryable && !retryAfterTooLong(resp.Header.Get("Retry-After")) {
				sleepBackoff(ctx, attempt, resp.Header.Get("Retry-After"))
				continue
			}
			return lastErr
		}

		if out != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, out); err != nil {
				return fmt.Errorf("quolle: decoding response: %w", err)
			}
		}
		return nil
	}
	return lastErr
}

func parseError(status int, raw []byte) *Error {
	e := &Error{StatusCode: status, Message: fmt.Sprintf("request failed with status %d", status)}
	var payload map[string]any
	if len(raw) > 0 && json.Unmarshal(raw, &payload) == nil {
		if msg, ok := payload["error"].(string); ok {
			e.Message = msg
			delete(payload, "error")
		}
		if code, ok := payload["code"].(string); ok {
			e.Code = code
			delete(payload, "code")
		}
		e.Data = payload
	}
	return e
}
