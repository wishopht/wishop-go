package wishop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const DefaultTolerance = 300 * time.Second

type webhookOptions struct {
	tolerance time.Duration
	now       func() time.Time
}

type WebhookOption func(*webhookOptions)

func WithTolerance(tolerance time.Duration) WebhookOption {
	return func(o *webhookOptions) { o.tolerance = tolerance }
}

func withClock(now func() time.Time) WebhookOption {
	return func(o *webhookOptions) { o.now = now }
}

func ConstructEvent(payload []byte, signatureHeader, secret string, opts ...WebhookOption) (*Event, error) {
	options := &webhookOptions{tolerance: DefaultTolerance, now: time.Now}
	for _, opt := range opts {
		opt(options)
	}

	timestamp, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return nil, err
	}

	if options.tolerance > 0 {
		age := options.now().Sub(timestamp)
		if age < 0 {
			age = -age
		}

		if age > options.tolerance {
			return nil, &SignatureVerificationError{Reason: "timestamp outside the tolerance window"}
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp.Unix(), 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	for _, signature := range signatures {
		decoded, err := hex.DecodeString(signature)
		if err != nil {
			continue
		}

		if hmac.Equal(decoded, expected) {
			event := &Event{}

			if err := json.Unmarshal(payload, event); err != nil {
				return nil, &SignatureVerificationError{Reason: "payload is not valid JSON"}
			}

			return event, nil
		}
	}

	return nil, &SignatureVerificationError{Reason: "no signature matched the signing secret"}
}

func parseSignatureHeader(header string) (time.Time, []string, error) {
	var (
		timestamp  time.Time
		signatures []string
		seen       bool
	)

	for _, part := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}

		switch key {
		case "t":
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return time.Time{}, nil, &SignatureVerificationError{Reason: "malformed timestamp"}
			}

			timestamp = time.Unix(seconds, 0)
			seen = true
		case "v1":
			if value != "" {
				signatures = append(signatures, value)
			}
		}
	}

	if !seen {
		return time.Time{}, nil, &SignatureVerificationError{Reason: "no timestamp in signature header"}
	}

	if len(signatures) == 0 {
		return time.Time{}, nil, &SignatureVerificationError{Reason: "no v1 signature in signature header"}
	}

	return timestamp, signatures, nil
}
