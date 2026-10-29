package quolle

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

// Domains, API keys and webhook endpoints (K8): each method hits the right route.

func recorder(t *testing.T) (*Client, *[]string, func()) {
	t.Helper()
	var seen []string
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if r.URL.RawQuery != "" {
			p += "?" + r.URL.RawQuery
		}
		seen = append(seen, r.Method+" "+p)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/v1/keys" {
			_, _ = w.Write([]byte(`{"key":"qle_new","id":"k2","name":"ci","scope":"sending","domains":[],"expiresAt":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"domains":[{"id":"d1"}],"domain":{"id":"d1"},"keys":[{"id":"k1","scope":"full"}],
			"webhooks":[],"deliveries":[],"events":[],"key":"qle_new","secret":"whsec_new","id":"x"}`))
	})
	return c, &seen, srv.Close
}

func TestDomainsRoutes(t *testing.T) {
	c, seen, done := recorder(t)
	defer done()
	ctx := context.Background()
	if ds, err := c.Domains.List(ctx); err != nil || ds[0].ID != "d1" {
		t.Fatalf("list: %v %v", ds, err)
	}
	if _, err := c.Domains.Create(ctx, "mail.acme.com"); err != nil {
		t.Fatal(err)
	}
	if d, err := c.Domains.Get(ctx, "d1"); err != nil || d.ID != "d1" {
		t.Fatalf("get: %v %v", d, err)
	}
	_, _ = c.Domains.Records(ctx, "d1")
	_, _ = c.Domains.Verify(ctx, "d1")
	_ = c.Domains.Remove(ctx, "d1")
	want := []string{"GET /v1/domains", "POST /v1/domains", "GET /v1/domains/d1", "GET /v1/domains/d1/records",
		"POST /v1/domains/d1/recheck", "DELETE /v1/domains/d1"}
	if !reflect.DeepEqual(*seen, want) {
		t.Errorf("routes = %v\nwant %v", *seen, want)
	}
}

func TestAPIKeysRoutes(t *testing.T) {
	c, seen, done := recorder(t)
	defer done()
	ctx := context.Background()
	if ks, err := c.APIKeys.List(ctx); err != nil || ks[0].ID != "k1" {
		t.Fatalf("list: %v %v", ks, err)
	}
	if k, err := c.APIKeys.Create(ctx, CreateAPIKeyParams{Name: "ci", Scope: ScopeSending}); err != nil || k.Key != "qle_new" {
		t.Fatalf("create: %v %v", k, err)
	}
	_ = c.APIKeys.Remove(ctx, "k1")
	want := []string{"GET /v1/keys", "POST /v1/keys", "DELETE /v1/keys/k1"}
	if !reflect.DeepEqual(*seen, want) {
		t.Errorf("routes = %v\nwant %v", *seen, want)
	}
}

func TestWebhookEndpointRoutes(t *testing.T) {
	c, seen, done := recorder(t)
	defer done()
	ctx := context.Background()
	_, _ = c.Webhooks.Events(ctx)
	_, _ = c.Webhooks.List(ctx)
	_, _ = c.Webhooks.Create(ctx, "https://x.io/h", []string{"email.delivered"})
	_, _ = c.Webhooks.Get(ctx, "w1")
	_, _ = c.Webhooks.Update(ctx, "w1", UpdateWebhookParams{Events: []string{"email.bounced"}})
	_, _ = c.Webhooks.Test(ctx, "w1")
	_, _ = c.Webhooks.Deliveries(ctx, "w1", "failed", 10)
	_ = c.Webhooks.Redeliver(ctx, "w1", "dl1")
	if s, err := c.Webhooks.RotateSecret(ctx, "w1"); err != nil || s != "whsec_new" {
		t.Fatalf("rotate: %q %v", s, err)
	}
	_ = c.Webhooks.Remove(ctx, "w1")
	want := []string{"GET /v1/webhooks/events", "GET /v1/webhooks", "POST /v1/webhooks", "GET /v1/webhooks/w1",
		"PATCH /v1/webhooks/w1", "POST /v1/webhooks/w1/test", "GET /v1/webhooks/w1/deliveries?limit=10&status=failed",
		"POST /v1/webhooks/w1/deliveries/dl1/redeliver", "POST /v1/webhooks/w1/rotate-secret", "DELETE /v1/webhooks/w1"}
	if !reflect.DeepEqual(*seen, want) {
		t.Errorf("routes = %v\nwant %v", *seen, want)
	}
}

func TestRestrictedKeyCode(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"code":"restricted_api_key","error":"This API key can only send email."}`))
	})
	defer srv.Close()
	_, err := c.APIKeys.List(context.Background())
	if qe, ok := err.(*Error); !ok || qe.Code != "restricted_api_key" || qe.StatusCode != 403 {
		t.Fatalf("err = %v", err)
	}
}
