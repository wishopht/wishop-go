package wishop

import "time"

const (
	ChargeStatusInitiated = "INITIATED"
	ChargeStatusOTPSent   = "OTP_SENT"
	ChargeStatusPaid      = "PAID"
	ChargeStatusFailed    = "FAILED"
	ChargeStatusExpired   = "EXPIRED"
	ChargeStatusRefunded  = "REFUNDED"
)

const (
	FailReasonInsufficientFunds   = "INSUFFICIENT_FUNDS"
	FailReasonPINLocked           = "PIN_LOCKED"
	FailReasonOTPExpired          = "OTP_EXPIRED"
	FailReasonOTPAttemptsExceeded = "OTP_ATTEMPTS_EXCEEDED"
	FailReasonWalletNotFound      = "WALLET_NOT_FOUND"
	FailReasonDeclined            = "DECLINED"
	FailReasonTimeout             = "TIMEOUT"
	FailReasonInternalError       = "INTERNAL_ERROR"
)

// ChargeChannel says where a charge came from: API charges are created by you
// and paid through hosted checkout; app charges are started by a customer in
// the WiShop app with your pay code.
const (
	ChargeChannelAPI = "api"
	ChargeChannelApp = "app"
)

const (
	FeeBearerMerchant = "merchant"
	FeeBearerCustomer = "customer"
)

const (
	RefundStatusPending   = "pending"
	RefundStatusSucceeded = "succeeded"
	RefundStatusFailed    = "failed"
)

const (
	RefundReasonRequestedByCustomer = "requested_by_customer"
	RefundReasonDuplicate           = "duplicate"
	RefundReasonFraudulent          = "fraudulent"
	RefundReasonOther               = "other"
)

const (
	PayoutStatusPending   = "PENDING"
	PayoutStatusSucceeded = "SUCCEEDED"
	PayoutStatusFailed    = "FAILED"
)

// Payout-only fail reasons. FailReasonWalletNotFound is shared with charges.
const (
	FailReasonInsufficientMerchantBalance = "INSUFFICIENT_MERCHANT_BALANCE"
	FailReasonRecipientIneligible         = "RECIPIENT_INELIGIBLE"
)

const (
	EventChargePaid     = "charge.paid"
	EventChargeFailed   = "charge.failed"
	EventChargeExpired  = "charge.expired"
	EventChargeRefunded = "charge.refunded"

	EventPayoutCreated   = "payout.created"
	EventPayoutSucceeded = "payout.succeeded"
	EventPayoutFailed    = "payout.failed"
)

type Charge struct {
	ID                  string `json:"id"`
	OrderID             string `json:"order_id"`
	Description         string `json:"description"`
	Currency            string `json:"currency"`
	Amount              string `json:"amount"`
	FeeBearer           string `json:"fee_bearer"`
	Fee                 string `json:"fee"`
	Net                 string `json:"net"`
	CustomerDebitAmount string `json:"customer_debit_amount"`
	AmountRefunded      string `json:"amount_refunded"`
	CustomerPhoneMasked string `json:"customer_phone_masked"`
	Channel             string `json:"channel"`
	Status              string `json:"status"`
	FailReason          string `json:"fail_reason"`
	ReturnURL           string `json:"return_url"`
	// Metadata values may be any JSON type; the API does not restrict them to strings.
	Metadata    map[string]any `json:"metadata"`
	Livemode    bool           `json:"livemode"`
	ExpiresAt   time.Time      `json:"expires_at"`
	ConfirmedAt time.Time      `json:"confirmed_at"`
	CreatedAt   time.Time      `json:"created_at"`
	PayURL      string         `json:"pay_url"`
}

type Refund struct {
	ID          string    `json:"id"`
	Charge      string    `json:"charge"`
	Amount      string    `json:"amount"`
	Currency    string    `json:"currency"`
	Reason      string    `json:"reason"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Livemode    bool      `json:"livemode"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateChargeParams struct {
	Amount      string         `json:"amount"`
	Currency    string         `json:"currency"`
	OrderID     string         `json:"order_id,omitempty"`
	Description string         `json:"description,omitempty"`
	FeeBearer   string         `json:"fee_bearer,omitempty"`
	ReturnURL   string         `json:"return_url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ListChargesParams struct {
	Limit         int
	StartingAfter string
	Status        string
	Query         string
}

type ChargeList struct {
	Charges []Charge `json:"charges"`
	HasMore bool     `json:"has_more"`
}

// Payout is money sent from your wallet to a customer's.
//
// Status is one of PayoutStatusSucceeded, PayoutStatusFailed, or
// PayoutStatusPending — and pending is not "queued": it means the wallet's
// answer was lost, so the money may or may not have moved. It resolves on its
// own and a payout.succeeded or payout.failed webhook reports the outcome.
// Never send a replacement payout for a pending one.
type Payout struct {
	ID             string `json:"id"`
	RecipientPhone string `json:"recipient_phone"`
	RecipientName  string `json:"recipient_name"`
	Currency       string `json:"currency"`
	Amount         string `json:"amount"`
	// Fee is charged on top of Amount: your wallet is debited Amount + Fee.
	Fee        string `json:"fee"`
	Status     string `json:"status"`
	FailReason string `json:"fail_reason"`
	Reference  string `json:"reference"`
	// Metadata values may be any JSON type, unlike a charge's string map.
	Metadata    map[string]any `json:"metadata"`
	Livemode    bool           `json:"livemode"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at"`
}

type CreatePayoutParams struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	// RecipientPhone is the customer's wallet number. There is no recipient
	// name field: the name is resolved from the wallet and returned.
	RecipientPhone string         `json:"recipient_phone"`
	Reference      string         `json:"reference,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ListPayoutsParams struct {
	Limit         int
	StartingAfter string
	Status        string
	Currency      string
}

type PayoutList struct {
	Payouts []Payout `json:"payouts"`
	HasMore bool     `json:"has_more"`
}

type CreateRefundParams struct {
	Amount      string `json:"amount,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Description string `json:"description,omitempty"`
}

type refundList struct {
	Refunds []Refund `json:"refunds"`
}

type Event struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	APIVersion string    `json:"api_version"`
	CreatedAt  time.Time `json:"created_at"`
	Livemode   bool      `json:"livemode"`
	Data       EventData `json:"data"`
}

// EventData carries the object an event is about. Object says which: "charge"
// on charge.* events, "payout" on payout.* ones.
//
// Charge stays a value rather than a pointer so existing code keeps compiling;
// on a payout event it is simply the zero Charge. Branch on Object (or on the
// event Type) rather than testing a field for emptiness.
type EventData struct {
	Object string  `json:"object"`
	Charge Charge  `json:"charge"`
	Refund *Refund `json:"refund"`
	// Payout is set only on payout.* events.
	Payout *Payout `json:"payout"`
}
