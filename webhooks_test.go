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

const payoutEventPayload = `{"id":"evt_6Lm4pT8vRw2nKxQd","type":"payout.succeeded","api_version":"2026-08-01","created_at":"2026-08-31T12:04:12.006Z","livemode":true,"data":{"object":"payout","payout":{"id":"po_7Hn2vB9xQm4tRkLw","recipient_phone":"+50931234567","currency":"HTG","amount":"150.00","fee":"3.00","status":"SUCCEEDED"}}}`

// A payout event carries no charge. Parsing must succeed and expose the payout
// rather than failing or silently producing a half-built charge.
func TestConstructEventParsesPayoutEvent(t *testing.T) {
	now := frozen()
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), sign(t, "whsec_test", now, payoutEventPayload))

	event, err := ConstructEvent([]byte(payoutEventPayload), header, "whsec_test", withClock(frozen))
	if err != nil {
		t.Fatalf("expected verification to succeed, got %v", err)
	}

	if event.Type != EventPayoutSucceeded {
		t.Errorf("event type = %q, want %q", event.Type, EventPayoutSucceeded)
	}

	if event.Data.Object != "payout" {
		t.Errorf("data.object = %q, want payout", event.Data.Object)
	}

	if event.Data.Payout == nil {
		t.Fatal("expected the payout to be present on a payout.succeeded event")
	}

	if event.Data.Payout.ID != "po_7Hn2vB9xQm4tRkLw" || event.Data.Payout.Fee != "3.00" {
		t.Errorf("payout = %+v", event.Data.Payout)
	}

	if event.Data.Payout.Status != PayoutStatusSucceeded {
		t.Errorf("payout status = %q", event.Data.Payout.Status)
	}

	// The charge is the zero value on a payout event; branch on Object, never
	// on a charge field being non-empty.
	if event.Data.Charge.ID != "" {
		t.Errorf("charge should be zero on a payout event, got %+v", event.Data.Charge)
	}
}
