# Wishop Go

The official Go client for the [Wishop API](https://developers.wishopht.com).

Create charges, send customers to a hosted checkout page, refund them, and verify webhooks. Standard library only — no dependencies.

## Install

```bash
go get github.com/wishop/wishop-go
```

Requires Go 1.21 or later.

## Quickstart

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/wishop/wishop-go"
)

func main() {
	client := wishop.New(os.Getenv("WISHOP_SECRET_KEY"))

	charge, err := client.Charges.Create(context.Background(), wishop.CreateChargeParams{
		Amount:      "1500.00",
		Currency:    "HTG",
		OrderID:     "order_1042",
		Description: "Two large coffees",
		ReturnURL:   "https://yourstore.com/orders/1042/complete",
		Metadata:    map[string]string{"cart_id": "c_88213"},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(charge.PayURL)
}
```

Send the customer to `charge.PayURL`. It is returned on create only, so persist it with the order.

When the payment reaches a terminal state, Wishop sends a webhook and returns the customer to your `ReturnURL`. Confirm the outcome from the webhook or by retrieving the charge — never from the redirect alone.

## Configuration

```go
client := wishop.New(
	os.Getenv("WISHOP_SECRET_KEY"),
	wishop.WithTimeout(10*time.Second),
	wishop.WithHTTPClient(myClient),
	wishop.WithBaseURL("http://localhost:8080"),
)
```

| Option | Default | Notes |
| --- | --- | --- |
| `WithTimeout` | 30s | Applied as a context deadline on every request |
| `WithHTTPClient` | `http.DefaultClient` | For proxies, custom transports, or connection tuning |
| `WithBaseURL` | `https://gatewayapi.wishopht.com` | For pointing at a mock server in your own tests |

Test and live mode are selected by the key, not by configuration. A `sk_test_` key reaches test data; a `sk_live_` key reaches live data.

Every method takes a `context.Context` first, so cancellation and deadlines from your request propagate.

## Charges

```go
charge, err := client.Charges.Create(ctx, params)
charge, err := client.Charges.Retrieve(ctx, "ch_3kQ8xR2mVn7pLdYt")
list, err := client.Charges.List(ctx, wishop.ListChargesParams{Status: wishop.ChargeStatusPaid})
```

`List` returns one page. Pass the id of the last charge as `StartingAfter` while `HasMore` is true:

```go
params := wishop.ListChargesParams{Limit: 100, Status: wishop.ChargeStatusPaid}

for {
	page, err := client.Charges.List(ctx, params)
	if err != nil {
		return err
	}

	for _, charge := range page.Charges {
		fulfil(charge)
	}

	if !page.HasMore {
		return nil
	}

	params.StartingAfter = page.Charges[len(page.Charges)-1].ID
}
```

## Refunds

```go
refund, err := client.Refunds.Create(ctx, "ch_3kQ8xR2mVn7pLdYt", wishop.CreateRefundParams{
	Amount: "500.00",
	Reason: wishop.RefundReasonRequestedByCustomer,
})

refunds, err := client.Refunds.List(ctx, "ch_3kQ8xR2mVn7pLdYt")
```

Leave `Amount` empty to refund everything still refundable. Refund lists are not paginated.

## Payouts

Sends money from your wallet to a customer's — payroll, marketplace settlements, rewards.

```go
payout, err := client.Payouts.Create(ctx, "payroll-2026-06-emp-4471", wishop.CreatePayoutParams{
	Amount:         "150.00",
	Currency:       "HTG",
	RecipientPhone: "+50931234567",
	Reference:      "June payroll",
})

payouts, err := client.Payouts.List(ctx, wishop.ListPayoutsParams{Status: wishop.PayoutStatusFailed})
one, err := client.Payouts.Retrieve(ctx, "po_7Hn2vB9xQm4tRkLw")
```

The idempotency key is a **required parameter**, not an option: a payout pushes money out, so an un-keyed retry after a dropped connection is indistinguishable from a second payment. Your wallet is debited `Amount + Fee`.

Three outcomes, and the middle one is the one to handle:

```go
switch payout.Status {
case wishop.PayoutStatusSucceeded:
	// The money moved.
case wishop.PayoutStatusPending:
	// The wallet's answer was lost — the money may or may not have moved.
	// It resolves on its own and a webhook reports the outcome.
	// Never send a replacement; retry with the SAME key if you must.
}
```

A declined payout returns an `*UnprocessableError` and still records a `FAILED` payout you can `Retrieve` — unlike a rejected refund, which leaves nothing behind.

## Money

Amounts are strings, never numbers, and every field that holds money is a string: `Amount`, `Fee`, `Net`, `CustomerDebitAmount`, `AmountRefunded`. Parse them with a decimal type, never a float.

```go
amount, err := decimal.NewFromString(charge.Amount)
```

## Idempotency

Pass a key on charge and refund creation so a retry after a timeout cannot charge twice:

```go
key := fmt.Sprintf("charge-%s-%d", order.ID, order.PaymentAttempt)

charge, err := client.Charges.Create(ctx, params, wishop.WithIdempotencyKey(key))
```

On charges and refunds nothing is sent unless you pass it. **Payouts are the exception: the key is a required parameter** and the API rejects a payout without one.

Derive a charge key from the payment *attempt*, not the order — reusing an order id replays the previous failure instead of creating the charge a retrying customer needs. Derive a payout key the opposite way, from the payment itself, and reuse it across retries: a fresh key on a payout retry sends a second payment. Keys are remembered for 24 hours.

## Errors

Every failure is a typed error. Use `errors.As`:

```go
charge, err := client.Charges.Create(ctx, params)

var rateLimit *wishop.RateLimitError
var unprocessable *wishop.UnprocessableError

switch {
case errors.As(err, &rateLimit):
	time.Sleep(rateLimit.RetryAfter)
case errors.As(err, &unprocessable):
	log.Println(unprocessable.Message())
case err != nil:
	return err
}
```

| Type | Status | Cause |
| --- | --- | --- |
| `ValidationError` | 400 | Malformed body, invalid amount or currency, metadata over the limits |
| `AuthenticationError` | 401 | Missing, malformed, unknown, or revoked key |
| `PermissionError` | 403 | Live key from an address that is not whitelisted |
| `NotFoundError` | 404 | No charge, refund or payout with that id in this mode; or no wallet for a payout recipient's phone |
| `ConflictError` | 409 | Idempotency key still in flight, charge no longer payable, or wallet not provisioned for live payouts |
| `ChargeExpiredError` | 410 | Charge expired |
| `PayloadTooLargeError` | 413 | Upload too large |
| `UnprocessableError` | 422 | Currency not enabled, charge not refundable, refund exceeds remaining, wallet declined, or a payout declined for balance or recipient |
| `RateLimitError` | 429 | Rate limit exceeded; carries `RetryAfter` |
| `APIError` | 5xx and anything unmapped | Failure on the gateway side |

All of them satisfy the `wishop.Error` interface, so `errors.As(err, &wishopErr)` catches any of them and gives you `StatusCode()` and `Message()`.

`Message()` is prose and its wording can change — branch on the type or the status code, never on the text.

The client does not retry. `ConflictError`, `RateLimitError`, and `APIError` are safe to retry with backoff and the same idempotency key; nothing else is.

A 422 on a payout is the one error that leaves state behind: the payout is recorded as `FAILED` and readable with `Payouts.Retrieve`.

## Payment failures

A failed payment is not an error — the call succeeded, the payment didn't. Read `Status` and `FailReason`:

```go
if charge.Status == wishop.ChargeStatusFailed {
	log.Println(explain(charge.FailReason))
}
```

| `FailReason` | Meaning |
| --- | --- |
| `INSUFFICIENT_FUNDS` | Wallet balance did not cover the payment |
| `PIN_LOCKED` | Wallet locked after repeated incorrect PINs |
| `OTP_EXPIRED` | One-time code expired before use |
| `OTP_ATTEMPTS_EXCEEDED` | Too many incorrect one-time codes |
| `WALLET_NOT_FOUND` | No wallet for that phone number |
| `DECLINED` | Wallet refused the payment |
| `TIMEOUT` | Confirmation timed out |
| `INTERNAL_ERROR` | Failure on our side |

Keep a default branch — new reasons can be added, and `FailReason` is a plain string so an unknown value arrives intact rather than failing to decode.

## Webhooks

Verification needs the body **exactly as received**. Read it before anything parses it:

```go
func handler(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := wishop.ConstructEvent(payload, r.Header.Get("X-Wishop-Signature"), os.Getenv("WISHOP_WEBHOOK_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch event.Type {
	case wishop.EventChargePaid:
		fulfil(event.Data.Charge)
	case wishop.EventChargeFailed:
		notifyFailure(event.Data.Charge)
	case wishop.EventChargeExpired:
		releaseStock(event.Data.Charge)
	case wishop.EventChargeRefunded:
		recordRefund(*event.Data.Refund)
	}

	w.WriteHeader(http.StatusOK)
}
```

Re-serializing parsed JSON changes whitespace and key order, and the signature will not match. Use the raw bytes.

`ConstructEvent` rejects a stale timestamp, an altered body, and a signature that does not match. It accepts a match on any `v1` value, which is what keeps events flowing during a secret rotation. The default tolerance is 300 seconds:

```go
event, err := wishop.ConstructEvent(payload, signature, secret, wishop.WithTolerance(60*time.Second))
```

Delivery is at-least-once, so deduplicate on the `X-Webhook-Id` header, which is stable across retries. Record it in the same transaction as the work it triggers. Respond `2xx` once the event is durably recorded and queue anything slow — you have about ten seconds per attempt.

`Data.Refund` is set only on `charge.refunded`. A partial refund leaves the charge `PAID`; read `AmountRefunded`.

## Testing

Test mode uses `sk_test_` keys against the same host. The outcome is chosen by a phone number — the one the customer enters for a charge, the recipient's for a payout. The two use different tables; `+50900000006` is how you exercise a pending payout. See [test phone numbers](https://developers.wishopht.com/testing/test-phone-numbers).

To test your own code against this client, point it at a local server:

```go
client := wishop.New("sk_test_key", wishop.WithBaseURL(server.URL))
```

## Reference

Field-by-field documentation for the charge, refund, payout, and event objects lives at [developers.wishopht.com](https://developers.wishopht.com).

## License

MIT
