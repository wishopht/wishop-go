# Changelog

## 1.2.0

- `Charge.Channel` (`ChargeChannelAPI` / `ChargeChannelApp`). Charges started by
  a customer in the WiShop app are returned by the API and carried by webhooks;
  the field was previously dropped.
- Fixed: charge `Metadata` is now `map[string]any`. The API accepts any JSON
  value, and a charge or webhook whose metadata held a number or boolean failed
  to decode into `map[string]string`. Code that reads `Metadata` values as
  strings needs a type assertion, and `CreateChargeParams.Metadata` takes
  `map[string]any`.

## 1.1.0

Payouts — send money from your wallet to a customer's.

- `Payouts.Create`, `Payouts.Retrieve`, `Payouts.List`
- `Payout`, `CreatePayoutParams`, `ListPayoutsParams`, `PayoutList` types
- `PayoutStatus*` constants, and `FailReasonInsufficientMerchantBalance` /
  `FailReasonRecipientIneligible`
- `EventPayoutCreated`, `EventPayoutSucceeded`, `EventPayoutFailed`

Fixed: `ConstructEvent` could not represent a `payout.*` event. `EventData` gains
a `Payout` field; on those events `Data.Charge` is the zero value, so branch on
`Data.Object` or the event type. Existing charge handling is unchanged.

Note `Payouts.Create` takes the idempotency key as a required parameter rather
than a `RequestOption` — the API requires one, and an un-keyed payout retry pays
twice.

## 1.0.0

Initial release.

- `Charges.Create`, `Charges.Retrieve`, `Charges.List`
- `Refunds.Create`, `Refunds.List`
- `ConstructEvent` for webhook signature verification
- Typed errors for every documented status code
