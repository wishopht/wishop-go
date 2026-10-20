# Wishop Go

Go client for the [Wishop API](https://developers.wishopht.com).

Use it to create charges, send customers to the hosted checkout page, issue refunds and payouts, and verify webhooks. It only uses the standard library.

## Install

```bash
go get github.com/wishopht/wishop-go
```

Requires Go 1.21 or later.

## Quickstart

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/wishopht/wishop-go"
)

func main() {
	client := wishop.New(os.Getenv("WISHOP_SECRET_KEY"))

	charge, err := client.Charges.Create(context.Background(), wishop.CreateChargeParams{
		Amount:      "1500.00",
		Currency:    "HTG",
		OrderID:     "order_1042",
		Description: "Two large coffees",
		ReturnURL:   "https://yourstore.com/orders/1042/complete",
		Metadata:    map[string]any{"cart_id": "c_88213"},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(charge.PayURL)
}
```

Redirect the customer to `charge.PayURL`. The API only returns it when the charge is created, so save it with the order.

When the payment finishes, Wishop sends you a webhook and redirects the customer to your `ReturnURL`. Use the webhook, or retrieve the charge, to confirm the result. Don't rely on the redirect alone.

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
| `WithTimeout` | 30s | Set as a context deadline on every request |
| `WithHTTPClient` | `http.DefaultClient` | For proxies, custom transports, or connection tuning |
| `WithBaseURL` | `https://gatewayapi.wishopht.com` | Change it to point at a mock server in your tests |

The key decides the mode. A `sk_test_` key uses test data and a `sk_live_` key uses live data.

Every method takes a `context.Context` as its first argument, so cancellations and deadlines from your own request carry through.

## Charges

```go
charge, err := client.Charges.Create(ctx, params)
charge, err := client.Charges.Retrieve(ctx, "ch_3kQ8xR2mVn7pLdYt")
list, err := client.Charges.List(ctx, wishop.ListChargesParams{Status: wishop.ChargeStatusPaid})
```

`List` returns one page at a time. To get the next page, pass the id of the last charge as `StartingAfter`, and stop when `HasMore` is false:

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

Set `Query` to search by charge id or order id. It is sent as the API's `q` parameter.

## Refunds

```go
refund, err := client.Refunds.Create(ctx, "ch_3kQ8xR2mVn7pLdYt", wishop.CreateRefundParams{
	Amount: "500.00",
	Reason: wishop.RefundReasonRequestedByCustomer,
})

refunds, err := client.Refunds.List(ctx, "ch_3kQ8xR2mVn7pLdYt")
```

Leave `Amount` empty to refund whatever is left on the charge. Refund lists are not paginated.

## Payouts

A payout sends money from your wallet to a customer's wallet, for example for payroll, marketplace settlements or rewards.

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

The idempotency key is the second argument of `Payouts.Create`, not an option, because payouts require one. Without it, retrying after a dropped connection could send the money twice. Your wallet is debited `Amount + Fee`.

A payout comes back with one of three statuses. Make sure your code handles `PayoutStatusPending`:

```go
switch payout.Status {
case wishop.PayoutStatusSucceeded:
	// The money was sent.
case wishop.PayoutStatusPending:
	// We didn't get a final answer from the wallet, so the money may or may not have moved.
	// It settles on its own and a webhook tells you the result.
	// Don't create a new payout. If you retry, use the same key.
}
```

If the payout is declined, `Create` returns an `*UnprocessableError`, but the payout is still saved as `FAILED` and you can `Retrieve` it. A rejected refund is not saved.

## Money

Every money field is a string: `Amount`, `Fee`, `Net`, `CustomerDebitAmount`, `AmountRefunded`. Parse them with a decimal type, not `float64`.

```go
amount, err := decimal.NewFromString(charge.Amount)
```

## Idempotency

Pass an idempotency key when creating charges and refunds, so retrying after a timeout can't charge the customer twice:

```go
key := fmt.Sprintf("charge-%s-%d", order.ID, order.PaymentAttempt)

charge, err := client.Charges.Create(ctx, params, wishop.WithIdempotencyKey(key))
```

For charges and refunds the key is optional. For payouts it is a required argument, and the API rejects a payout sent with an empty key.

Build a charge key from the payment attempt, not just the order. If you reuse the order id, a customer who tries again gets the earlier failed charge back instead of a new one. For payouts, do the opposite: build the key from the payout itself and reuse it on every retry, because a new key creates a second payout. Keys expire after 24 hours.

## Errors

Every failure returns a typed error. Check it with `errors.As`:

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
| `ValidationError` | 400 | Malformed body, invalid amount or currency, or metadata over the limits |
| `AuthenticationError` | 401 | Missing, malformed, unknown, or revoked key |
| `PermissionError` | 403 | Live key used from an IP address that is not whitelisted |
| `NotFoundError` | 404 | No charge, refund or payout with that id in this mode, or no wallet for the payout recipient's phone |
| `ConflictError` | 409 | A request with the same idempotency key is still running, the charge can no longer be paid, or your wallet is not set up for live payouts |
| `ChargeExpiredError` | 410 | The charge has expired |
| `PayloadTooLargeError` | 413 | Request body too large |
| `UnprocessableError` | 422 | Currency not enabled, charge not refundable, refund larger than what is left, wallet declined, or payout declined for balance or recipient |
| `RateLimitError` | 429 | Too many requests. `RetryAfter` gives the wait |
| `APIError` | 5xx or anything else | Server error on Wishop's side |

They all implement the `wishop.Error` interface, so `errors.As(err, &wishopErr)` matches any of them and gives you `StatusCode()` and `Message()`.

The wording of `Message()` can change. Check the error type or the status code in your code, not the message text.

The client never retries on its own. You can retry `ConflictError`, `RateLimitError` and `APIError` with backoff, using the same idempotency key. Don't retry the others.

## Payment failures

A failed payment doesn't return an error. The API call worked; the customer's payment did not. Check `Status` and `FailReason`:

```go
if charge.Status == wishop.ChargeStatusFailed {
	log.Println(explain(charge.FailReason))
}
```

| `FailReason` | Meaning |
| --- | --- |
| `INSUFFICIENT_FUNDS` | Not enough money in the wallet |
| `PIN_LOCKED` | Wallet locked after too many wrong PINs |
| `OTP_EXPIRED` | The one-time code expired |
| `OTP_ATTEMPTS_EXCEEDED` | Too many wrong one-time codes |
| `WALLET_NOT_FOUND` | No wallet for that phone number |
| `DECLINED` | The wallet declined the payment |
| `TIMEOUT` | The customer didn't confirm in time |
| `INTERNAL_ERROR` | Something failed on Wishop's side |

Always have a `default` case for reasons you don't recognise, because new ones may be added. `FailReason` is a plain string, so an unknown value still decodes.

## Webhooks

Verify the signature against the raw request body. Read the bytes before anything parses them:

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
	case wishop.EventPayoutSucceeded:
		markPayoutSent(*event.Data.Payout)
	case wishop.EventPayoutFailed:
		markPayoutFailed(*event.Data.Payout)
	}

	w.WriteHeader(http.StatusOK)
}
```

Don't decode the JSON and encode it again before verifying. That changes the bytes, and the signature won't match.

`ConstructEvent` returns an error if the timestamp is too old, the body was changed, or the signature doesn't match. A header can carry several `v1` signatures and one match is enough, so events keep arriving while you rotate your secret. Timestamps up to 300 seconds old are accepted by default. To change that:

```go
event, err := wishop.ConstructEvent(payload, signature, secret, wishop.WithTolerance(60*time.Second))
```

The same event can arrive more than once, so check the `X-Webhook-Id` header, which stays the same on every retry. Save it in the same database transaction as the work the event triggers. Reply with `2xx` as soon as you have saved the event, and do slow work in the background. Each delivery attempt times out after about ten seconds.

`Data.Refund` is only set on `charge.refunded`, and `Data.Payout` only on `payout.*` events. `Data.Object` tells you which kind of event you have. After a partial refund the charge stays `PAID`; read `AmountRefunded` to see how much was returned.

## Testing

Test mode uses `sk_test_` keys on the same API host. The phone number decides what happens: the customer's number for a charge, the recipient's for a payout. Charges and payouts use different test numbers; `+50900000006` gives you a pending payout. See [test phone numbers](https://developers.wishopht.com/testing/test-phone-numbers).

To test your own code without calling Wishop, point the client at a local server:

```go
client := wishop.New("sk_test_key", wishop.WithBaseURL(server.URL))
```

## Reference

Every field on charges, refunds, payouts and events is documented at [developers.wishopht.com](https://developers.wishopht.com).

## License

MIT
