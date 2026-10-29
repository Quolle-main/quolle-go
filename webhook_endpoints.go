package quolle

import (
	"context"
	"net/url"
	"strconv"
)

// Webhooks manages webhook endpoints (client.Webhooks). Needs a full-access
// key. To check an incoming delivery's signature, use VerifyWebhook.
type Webhooks struct{ client *Client }

// Webhook is an endpoint. Secret is masked when read back.
type Webhook struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    *string  `json:"secret"`
	CreatedAt string   `json:"createdAt"`
}

// CreatedWebhook holds the new endpoint. Secret (whsec_…) is shown once; store it.
type CreatedWebhook struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

// UpdateWebhookParams changes an endpoint. Nil fields are left as they are.
type UpdateWebhookParams struct {
	URL    *string  `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
}

// WebhookTestResult is how the endpoint answered a test event.
type WebhookTestResult struct {
	EventID string  `json:"eventId"`
	OK      bool    `json:"ok"`
	Status  *int    `json:"status"`
	Ms      *int    `json:"ms"`
	Error   *string `json:"error"`
}

// WebhookDelivery is one event sent to an endpoint.
type WebhookDelivery struct {
	ID             string         `json:"id"`
	EventID        string         `json:"eventId"`
	Event          string         `json:"event"`
	Status         string         `json:"status"` // succeeded, pending or failed
	Attempts       int            `json:"attempts"`
	ResponseStatus *int           `json:"responseStatus"`
	ResponseMs     *int           `json:"responseMs"`
	LastError      *string        `json:"lastError"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

// WebhookEventType is an event a webhook can subscribe to.
type WebhookEventType struct {
	Event string `json:"event"`
	Group string `json:"group"`
}

func webhookPath(id string) string { return "/v1/webhooks/" + url.PathEscape(id) }

// Events lists the events a webhook can subscribe to.
func (w *Webhooks) Events(ctx context.Context) ([]WebhookEventType, error) {
	var out struct {
		Events []WebhookEventType `json:"events"`
	}
	err := w.client.do(ctx, "GET", "/v1/webhooks/events", nil, nil, &out)
	return out.Events, err
}

// List returns the endpoints.
func (w *Webhooks) List(ctx context.Context) ([]Webhook, error) {
	var out struct {
		Webhooks []Webhook `json:"webhooks"`
	}
	err := w.client.do(ctx, "GET", "/v1/webhooks", nil, nil, &out)
	return out.Webhooks, err
}

// Create adds an endpoint. The returned Secret is shown once; store it.
func (w *Webhooks) Create(ctx context.Context, endpoint string, events []string) (*CreatedWebhook, error) {
	var out CreatedWebhook
	body := map[string]any{"url": endpoint, "events": events}
	if err := w.client.do(ctx, "POST", "/v1/webhooks", body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one endpoint.
func (w *Webhooks) Get(ctx context.Context, id string) (*Webhook, error) {
	var out Webhook
	if err := w.client.do(ctx, "GET", webhookPath(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update changes the URL or events. The signing secret stays the same.
func (w *Webhooks) Update(ctx context.Context, id string, params UpdateWebhookParams) (*Webhook, error) {
	var out Webhook
	if err := w.client.do(ctx, "PATCH", webhookPath(id), params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes an endpoint.
func (w *Webhooks) Remove(ctx context.Context, id string) error {
	return w.client.do(ctx, "DELETE", webhookPath(id), nil, nil, nil)
}

// Test sends a signed webhook.test event now and reports how the endpoint answered.
func (w *Webhooks) Test(ctx context.Context, id string) (*WebhookTestResult, error) {
	var out WebhookTestResult
	if err := w.client.do(ctx, "POST", webhookPath(id)+"/test", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Deliveries returns deliveries from the last 30 days, newest first. Status
// may be "", "succeeded", "pending" or "failed"; limit 0 means the default.
func (w *Webhooks) Deliveries(ctx context.Context, id, status string, limit int) ([]WebhookDelivery, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := webhookPath(id) + "/deliveries"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out struct {
		Deliveries []WebhookDelivery `json:"deliveries"`
	}
	err := w.client.do(ctx, "GET", path, nil, nil, &out)
	return out.Deliveries, err
}

// Redeliver sends a delivery again, with the same body and event id.
func (w *Webhooks) Redeliver(ctx context.Context, id, deliveryID string) error {
	return w.client.do(ctx, "POST", webhookPath(id)+"/deliveries/"+url.PathEscape(deliveryID)+"/redeliver", nil, nil, nil)
}

// RotateSecret replaces the signing secret. It takes effect immediately and
// the new secret is returned once.
func (w *Webhooks) RotateSecret(ctx context.Context, id string) (string, error) {
	var out struct {
		Secret string `json:"secret"`
	}
	err := w.client.do(ctx, "POST", webhookPath(id)+"/rotate-secret", nil, nil, &out)
	return out.Secret, err
}
