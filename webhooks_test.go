package wishop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
)

const eventPayload = `{"id":"evt_7Fk2mQ9xLp4nRt8W","type":"charge.refunded","api_version":"2026-08-01","created_at":"2026-08-12T17:22:08.559Z","livemode":true,"data":{"object":"charge","charge":{"id":"ch_3kQ8xR2mVn7pLdYt","status":"PAID","amount":"1500.00","amount_refunded":"500.00"},"refund":{"id":"re_8Wq3nK6yTf2mBxLp","charge":"ch_3kQ8xR2mVn7pLdYt","amount":"500.00","status":"succeeded"}}}`

func sign(t *testing.T, secret string, timestamp time.Time, payload string) string {
	t.Helper()

	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", timestamp.Unix(), payload)

	return hex.EncodeToString(mac.Sum(nil))
}

func frozen() time.Time {
	return time.Unix(1754990400, 0)
}

func TestConstructEventReturnsEvent(t *testing.T) {
	now := frozen()
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), sign(t, "whsec_test", now, eventPayload))

	event, err := ConstructEvent([]byte(eventPayload), header, "whsec_test", withClock(frozen))
	if err != nil {
		t.Fatalf("expected verification to succeed, got %v", err)
	}

	if event.ID != "evt_7Fk2mQ9xLp4nRt8W" {
		t.Errorf("event id = %q", event.ID)
	}

	if event.Type != EventChargeRefunded {
		t.Errorf("event type = %q", event.Type)
	}

	if event.Data.Refund == nil || event.Data.Refund.ID != "re_8Wq3nK6yTf2mBxLp" {
		t.Error("expected the refund to be present on a charge.refunded event")
	}

	if event.Data.Charge.AmountRefunded != "500.00" {
		t.Errorf("amount refunded = %q", event.Data.Charge.AmountRefunded)
	}
}

func TestConstructEventAcceptsSecondSignatureDuringRotation(t *testing.T) {
	now := frozen()
	header := fmt.Sprintf(
		"t=%d,v1=%s,v1=%s",
		now.Unix(),
		sign(t, "whsec_previous", now, eventPayload),
		sign(t, "whsec_current", now, eventPayload),
	)

	if _, err := ConstructEvent([]byte(eventPayload), header, "whsec_current", withClock(frozen)); err != nil {
		t.Fatalf("expected the second v1 to be accepted, got %v", err)
	}
}

func TestConstructEventRejectsTimestampOutsideTolerance(t *testing.T) {
	stale := frozen().Add(-10 * time.Minute)
	header := fmt.Sprintf("t=%d,v1=%s", stale.Unix(), sign(t, "whsec_test", stale, eventPayload))

	_, err := ConstructEvent([]byte(eventPayload), header, "whsec_test", withClock(frozen))

	var verificationErr *SignatureVerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("expected a signature verification error, got %v", err)
	}
}

func TestConstructEventAcceptsStaleTimestampWhenToleranceDisabled(t *testing.T) {
	stale := frozen().Add(-10 * time.Minute)
	header := fmt.Sprintf("t=%d,v1=%s", stale.Unix(), sign(t, "whsec_test", stale, eventPayload))

	if _, err := ConstructEvent([]byte(eventPayload), header, "whsec_test", withClock(frozen), WithTolerance(0)); err != nil {
		t.Fatalf("expected tolerance to be skipped, got %v", err)
	}
}

func TestConstructEventRejectsAlteredPayload(t *testing.T) {
	now := frozen()
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), sign(t, "whsec_test", now, eventPayload))
	altered := eventPayload[:len(eventPayload)-1] + " }"

	if _, err := ConstructEvent([]byte(altered), header, "whsec_test", withClock(frozen)); err == nil {
		t.Fatal("expected an altered payload to be rejected")
	}
}

func TestConstructEventRejectsWrongSecret(t *testing.T) {
	now := frozen()
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), sign(t, "whsec_other", now, eventPayload))

	if _, err := ConstructEvent([]byte(eventPayload), header, "whsec_test", withClock(frozen)); err == nil {
		t.Fatal("expected a mismatched secret to be rejected")
	}
}

func TestConstructEventRejectsMalformedHeaders(t *testing.T) {
	headers := map[string]string{
		"empty":             "",
		"garbage":           "not-a-signature",
		"missing v1":        "t=1754990400",
		"missing t":         "v1=abc123",
		"unparsable t":      "t=later,v1=abc123",
		"empty v1":          "t=1754990400,v1=",
		"non hex v1":        "t=1754990400,v1=zzzz",
		"only separators":   ",,,",
		"v2 signature only": "t=1754990400,v2=abc123",
	}

	for name, header := range headers {
		t.Run(name, func(t *testing.T) {
			_, err := ConstructEvent([]byte(eventPayload), header, "whsec_test", withClock(frozen))

			var verificationErr *SignatureVerificationError
			if !errors.As(err, &verificationErr) {
				t.Fatalf("expected a signature verification error, got %v", err)
			}
		})
	}
}
