# Quolle Go SDK

Official Go client for the [Quolle](https://quolle.com) email API.

## Install

```bash
go get github.com/Quolle-main/quolle-go
```

Requires Go 1.21+. No third-party dependencies.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Quolle-main/quolle-go"
)

func main() {
	client := quolle.New("qle_...") // or set QUOLLE_API_KEY

	res, err := client.Emails.Send(context.Background(), quolle.SendParams{
		From:    "hello@mail.yourdomain.com",
		To:      quolle.To("customer@example.com"),
		Subject: "Welcome!",
		HTML:    "<h1>Thanks for signing up</h1>",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Queued:", res.ID)
}
```

## Sending

### Multiple recipients

```go
client.Emails.Send(ctx, quolle.SendParams{
	From:    "hello@mail.yourdomain.com",
	To:      quolle.ToMany("a@example.com", "b@example.com"),
	Subject: "Announcement",
	HTML:    "<p>Hello everyone</p>",
})
```

### Templates

```go
client.Emails.Send(ctx, quolle.SendParams{
	From:      "hello@mail.yourdomain.com",
	To:        quolle.To("customer@example.com"),
	Template:  "welcome-email",
	Variables: map[string]any{"firstName": "Amaka", "planName": "Starter"},
})
```

### Scheduled send

```go
client.Emails.Send(ctx, quolle.SendParams{
	From:        "hello@mail.yourdomain.com",
	To:          quolle.To("customer@example.com"),
	Subject:     "Your weekly digest",
	HTML:        "<p>Here's what happened this week.</p>",
	ScheduledAt: "2026-12-25T09:00:00.000Z",
})
```

### Idempotency

```go
client.Emails.Send(ctx, params, quolle.WithIdempotencyKey("order_invoice_12345"))
```

### Batch

```go
out, _ := client.Emails.SendBatch(ctx, []quolle.SendParams{
	{From: "hello@mail.yourdomain.com", To: quolle.To("a@example.com"), Subject: "Hi Alice", HTML: "<p>Hi Alice</p>"},
	{From: "hello@mail.yourdomain.com", To: quolle.To("b@example.com"), Subject: "Hi Bob", HTML: "<p>Hi Bob</p>"},
})
fmt.Printf("Queued %d: %v\n", out.Queued, out.IDs)
```

### Attachments

Attach files with the `Attachments` field. `quolle.FileAttachment` reads a file and
base64-encodes it (guessing the content type). Up to 20 files, 10 MB total.

```go
pdf, err := quolle.FileAttachment("invoice.pdf")
if err != nil {
	log.Fatal(err)
}
client.Emails.Send(ctx, quolle.SendParams{
	From:        "billing@mail.yourdomain.com",
	To:          quolle.To("customer@example.com"),
	Subject:     "Your invoice",
	HTML:        "<p>Invoice attached.</p>",
	Attachments: []quolle.Attachment{pdf},
})
```

## Retrieve & cancel

```go
email, _ := client.Emails.Get(ctx, "a1b2c3d4-...")
fmt.Println(email.Status) // queued | sending | sent | delivered | bounced | failed

client.Emails.Cancel(ctx, "a1b2c3d4-...") // only while status == "scheduled"
```

## Error handling

```go
res, err := client.Emails.Send(ctx, params)
if err != nil {
	var qe *quolle.Error
	if errors.As(err, &qe) {
		fmt.Println(qe.StatusCode) // e.g. 402
		fmt.Println(qe.Code)       // e.g. "monthly_quota_exceeded" — branch on this
		fmt.Println(qe.Message)    // e.g. "Monthly limit reached"
		fmt.Println(qe.Data)       // extra fields, e.g. map[limit:3000]
	}
}
```

## Testing your integration

Send to a reserved test address to simulate any outcome without touching your
sending reputation: `delivered@test.quolle.com`, `bounced@test.quolle.com`,
`complained@test.quolle.com`, `suppressed@test.quolle.com`.

## Verifying webhooks

Confirm an incoming webhook really came from Quolle with the raw body, the `Quolle-Signature` header, and your signing secret (`whsec_…`):

```go
func handler(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    if err := quolle.VerifyWebhook(body, r.Header.Get("Quolle-Signature"), "whsec_...", 0); err != nil {
        http.Error(w, "invalid signature", http.StatusBadRequest)
        return
    }
    // signature valid — parse body and handle the event
}
```

HMAC-SHA256 with a 5-minute timestamp window (replay protection). Pass `0` for the default tolerance.

## Automatic retries

Transient failures — HTTP 429 (rate limit) and 5xx, plus network errors — are retried automatically with exponential backoff, honoring the `Retry-After` header. To avoid double-sending, a POST is only retried on a 5xx/network error when you pass an idempotency key; a 429 is always safe to retry (the request was never processed). Tune with `quolle.WithMaxRetries(n)` (default 3).

## License

MIT
# quolle-go
