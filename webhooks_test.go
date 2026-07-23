package quolle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func sign(secret string, ts int64, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(fmt.Sprintf("%d.%s", ts, body)))
	return hex.EncodeToString(m.Sum(nil))
}

func TestVerifyWebhookValid(t *testing.T) {
	secret := "whsec_test"
	body := `{"event":"email.delivered"}`
	ts := time.Now().Unix()
	header := fmt.Sprintf("t=%d,v1=%s", ts, sign(secret, ts, body))

	if err := VerifyWebhook([]byte(body), header, secret, 0); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyWebhookRejectsTamper(t *testing.T) {
	secret := "whsec_test"
	body := `{"event":"email.delivered"}`
	ts := time.Now().Unix()
	header := fmt.Sprintf("t=%d,v1=%s", ts, sign(secret, ts, body))

	if err := VerifyWebhook([]byte(body+"x"), header, secret, 0); err == nil {
		t.Error("tampered body should be rejected")
	}
}

func TestVerifyWebhookRejectsStaleTimestamp(t *testing.T) {
	secret := "whsec_test"
	body := `{"event":"email.delivered"}`
	old := time.Now().Unix() - 10000
	header := fmt.Sprintf("t=%d,v1=%s", old, sign(secret, old, body))

	if err := VerifyWebhook([]byte(body), header, secret, 0); err == nil {
		t.Error("stale timestamp should be rejected")
	}
}

func TestVerifyWebhookRejectsWrongSecret(t *testing.T) {
	body := `{"event":"email.delivered"}`
	ts := time.Now().Unix()
	header := fmt.Sprintf("t=%d,v1=%s", ts, sign("whsec_right", ts, body))

	if err := VerifyWebhook([]byte(body), header, "whsec_wrong", 0); err == nil {
		t.Error("wrong secret should be rejected")
	}
}
