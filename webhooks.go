package quolle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is the max age of a webhook timestamp before it's
// rejected as a possible replay.
const DefaultWebhookTolerance = 5 * time.Minute

// VerifyWebhook checks that a webhook payload really came from Quolle.
//
// Pass the RAW request body, the value of the "Quolle-Signature" header, and
// your webhook signing secret (whsec_…, shown once when you created the
// webhook). Returns nil on success, or an error if the header is malformed, the
// timestamp is outside the tolerance window, or the signature doesn't match.
//
//	if err := quolle.VerifyWebhook(body, r.Header.Get("Quolle-Signature"), secret, 0); err != nil {
//	    http.Error(w, "invalid signature", http.StatusBadRequest)
//	    return
//	}
func VerifyWebhook(payload []byte, signatureHeader, secret string, tolerance time.Duration) error {
	if tolerance <= 0 {
		tolerance = DefaultWebhookTolerance
	}

	var tStr, v1 string
	for _, part := range strings.Split(signatureHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			tStr = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if tStr == "" || v1 == "" {
		return &Error{Message: "invalid Quolle-Signature header"}
	}

	ts, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return &Error{Message: "invalid timestamp in Quolle-Signature header"}
	}
	age := time.Since(time.Unix(ts, 0))
	if age < 0 {
		age = -age
	}
	if age > tolerance {
		return &Error{Message: "webhook timestamp outside the tolerance window (possible replay)"}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, payload)))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(v1)) {
		return &Error{Message: "webhook signature verification failed"}
	}
	return nil
}
