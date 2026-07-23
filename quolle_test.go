package quolle

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient points the SDK at a local test server.
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := New("qle_test", WithBaseURL(srv.URL))
	return c, srv
}

func TestSend(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotIdem, gotMethod, gotPath string

	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotIdem = r.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"em_1","message":"Email queued successfully"}`))
	})
	defer srv.Close()

	res, err := c.Emails.Send(context.Background(), SendParams{
		From:    "hello@mail.acme.com",
		To:      To("user@example.com"),
		Subject: "Hi",
		HTML:    "<p>Hi</p>",
	}, WithIdempotencyKey("order_123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ID != "em_1" {
		t.Errorf("id = %q, want em_1", res.ID)
	}
	if gotMethod != "POST" || gotPath != "/v1/emails/send" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer qle_test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotIdem != "order_123" {
		t.Errorf("idempotency = %q", gotIdem)
	}
	if gotBody["from"] != "hello@mail.acme.com" {
		t.Errorf("from = %v", gotBody["from"])
	}
	// single recipient must serialize as a bare string, not an array
	if _, isString := gotBody["to"].(string); !isString {
		t.Errorf("to should be a string for one recipient, got %T", gotBody["to"])
	}
}

func TestSendManyRecipientsSerializesAsArray(t *testing.T) {
	var gotBody map[string]any
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":"em_1"}`))
	})
	defer srv.Close()

	_, err := c.Emails.Send(context.Background(), SendParams{
		From:    "a@b.com",
		To:      ToMany("x@y.com", "z@y.com"),
		Subject: "s",
		HTML:    "<p>x</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, isArr := gotBody["to"].([]any); !isArr {
		t.Errorf("to should be an array for many recipients, got %T", gotBody["to"])
	}
}

func TestSendBatch(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/emails/batch" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"queued":2,"ids":["a","b"]}`))
	})
	defer srv.Close()

	out, err := c.Emails.SendBatch(context.Background(), []SendParams{
		{From: "a@b.com", To: To("x@y.com"), Subject: "1", HTML: "<p>1</p>"},
		{From: "a@b.com", To: To("z@y.com"), Subject: "2", HTML: "<p>2</p>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Queued != 2 || len(out.IDs) != 2 {
		t.Errorf("got %+v", out)
	}
}

func TestGetUnwrapsEmail(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":{"id":"em_1","status":"delivered","opensCount":3}}`))
	})
	defer srv.Close()

	email, err := c.Emails.Get(context.Background(), "em_1")
	if err != nil {
		t.Fatal(err)
	}
	if email.Status != "delivered" || email.OpensCount != 3 {
		t.Errorf("got %+v", email)
	}
}

func TestCancel(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"message":"Scheduled email cancelled"}`))
	})
	defer srv.Close()

	out, err := c.Emails.Cancel(context.Background(), "em_1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != "Scheduled email cancelled" {
		t.Errorf("message = %q", out.Message)
	}
}

func TestErrorResponse(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		_, _ = w.Write([]byte(`{"error":"Monthly limit reached","limit":3000}`))
	})
	defer srv.Close()

	_, err := c.Emails.Send(context.Background(), SendParams{
		From: "a@b.com", To: To("c@d.com"), Subject: "s", HTML: "<p>x</p>",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	qe, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *quolle.Error, got %T", err)
	}
	if qe.StatusCode != 402 || qe.Message != "Monthly limit reached" {
		t.Errorf("got %+v", qe)
	}
	if qe.Data["limit"].(float64) != 3000 {
		t.Errorf("limit = %v", qe.Data["limit"])
	}
}

func TestMissingKey(t *testing.T) {
	t.Setenv("QUOLLE_API_KEY", "")
	c := New("")
	_, err := c.Emails.Send(context.Background(), SendParams{From: "a@b.com", To: To("c@d.com"), Subject: "s", HTML: "<p>x</p>"})
	if err == nil {
		t.Fatal("expected an error for missing key")
	}
}

func TestRetryOn429ThenSucceeds(t *testing.T) {
	var n int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 3 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"em_ok"}`))
	})
	defer srv.Close()
	c.maxRetries = 3

	res, err := c.Emails.Send(context.Background(), SendParams{From: "a@b.com", To: To("c@d.com"), Subject: "s", HTML: "<p>x</p>"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "em_ok" || n != 3 {
		t.Errorf("id=%q calls=%d (want em_ok, 3)", res.ID, n)
	}
}

func TestNoRetry5xxOnPostWithoutIdempotency(t *testing.T) {
	var n int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	defer srv.Close()
	c.maxRetries = 3

	_, err := c.Emails.Send(context.Background(), SendParams{From: "a@b.com", To: To("c@d.com"), Subject: "s", HTML: "<p>x</p>"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if n != 1 {
		t.Errorf("POST 5xx without idempotency must not retry; calls=%d (want 1)", n)
	}
}

func TestRetry5xxOnPostWithIdempotency(t *testing.T) {
	var n int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":"later"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"em_ok"}`))
	})
	defer srv.Close()
	c.maxRetries = 3

	res, err := c.Emails.Send(context.Background(), SendParams{From: "a@b.com", To: To("c@d.com"), Subject: "s", HTML: "<p>x</p>"}, WithIdempotencyKey("order_1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "em_ok" || n != 2 {
		t.Errorf("id=%q calls=%d (want em_ok, 2)", res.ID, n)
	}
}
