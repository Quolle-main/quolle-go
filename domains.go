package quolle

import (
	"context"
	"net/url"
)

// Domains manages sending domains (client.Domains). Needs a full-access API key.
type Domains struct{ client *Client }

// DomainWarmup is a verified domain's 14-day warm-up.
type DomainWarmup struct {
	Capped      bool   `json:"capped"`
	Day         int    `json:"day"`
	GraduatesAt string `json:"graduatesAt,omitempty"`
	DailyLimits *struct {
		Transactional int `json:"transactional"`
		Marketing     int `json:"marketing"`
	} `json:"dailyLimits,omitempty"`
	SentToday *struct {
		Transactional int `json:"transactional"`
		Marketing     int `json:"marketing"`
	} `json:"sentToday,omitempty"`
	ResetsAt string `json:"resetsAt,omitempty"`
}

// Domain is a sending domain.
type Domain struct {
	ID                     string        `json:"id"`
	Domain                 string        `json:"domain"`
	Status                 string        `json:"status"` // pending, verified or failed
	DNSProvider            *string       `json:"dnsProvider"`
	DNSPublished           bool          `json:"dnsPublished"`
	VerificationError      *string       `json:"verificationError"`
	TrackingEnabled        bool          `json:"trackingEnabled"`
	TrackingDomain         *string       `json:"trackingDomain"`
	TrackingDomainVerified bool          `json:"trackingDomainVerified"`
	CreatedAt              string        `json:"createdAt"`
	Warmup                 *DomainWarmup `json:"warmup,omitempty"`
}

// CreateDomainResult is the new domain and the DNS records to publish.
type CreateDomainResult struct {
	Domain       Domain           `json:"domain"`
	DNSRecords   []map[string]any `json:"dnsRecords"`
	IsCloudflare bool             `json:"isCloudflare"`
	Message      string           `json:"message"`
}

// List returns every sending domain, with verification and warm-up state.
func (d *Domains) List(ctx context.Context) ([]Domain, error) {
	var out struct {
		Domains []Domain `json:"domains"`
	}
	if err := d.client.do(ctx, "GET", "/v1/domains", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Domains, nil
}

// Create adds a domain (e.g. "mail.example.com") and returns the DNS records to publish.
func (d *Domains) Create(ctx context.Context, domain string) (*CreateDomainResult, error) {
	var out CreateDomainResult
	if err := d.client.do(ctx, "POST", "/v1/domains", map[string]string{"domain": domain}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one domain.
func (d *Domains) Get(ctx context.Context, id string) (*Domain, error) {
	var out struct {
		Domain Domain `json:"domain"`
	}
	if err := d.client.do(ctx, "GET", "/v1/domains/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Domain, nil
}

// Records returns the DNS records to publish, and which of them are found.
func (d *Domains) Records(ctx context.Context, id string) (map[string]any, error) {
	out := map[string]any{}
	err := d.client.do(ctx, "GET", "/v1/domains/"+url.PathEscape(id)+"/records", nil, nil, &out)
	return out, err
}

// Verify checks DNS again now instead of waiting for the next automatic check.
func (d *Domains) Verify(ctx context.Context, id string) (map[string]any, error) {
	out := map[string]any{}
	err := d.client.do(ctx, "POST", "/v1/domains/"+url.PathEscape(id)+"/recheck", nil, nil, &out)
	return out, err
}

// Remove deletes a domain. Mail can no longer be sent from it.
func (d *Domains) Remove(ctx context.Context, id string) error {
	return d.client.do(ctx, "DELETE", "/v1/domains/"+url.PathEscape(id), nil, nil, nil)
}
