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
	EventChargePaid     = "charge.paid"
	EventChargeFailed   = "charge.failed"
	EventChargeExpired  = "charge.expired"
	EventChargeRefunded = "charge.refunded"
)

type Charge struct {
	ID                  string            `json:"id"`
	OrderID             string            `json:"order_id"`
	Description         string            `json:"description"`
	Currency            string            `json:"currency"`
	Amount              string            `json:"amount"`
	FeeBearer           string            `json:"fee_bearer"`
	Fee                 string            `json:"fee"`
	Net                 string            `json:"net"`
	CustomerDebitAmount string            `json:"customer_debit_amount"`
	AmountRefunded      string            `json:"amount_refunded"`
	CustomerPhoneMasked string            `json:"customer_phone_masked"`
	Status              string            `json:"status"`
	FailReason          string            `json:"fail_reason"`
	ReturnURL           string            `json:"return_url"`
	Metadata            map[string]string `json:"metadata"`
	Livemode            bool              `json:"livemode"`
	ExpiresAt           time.Time         `json:"expires_at"`
	ConfirmedAt         time.Time         `json:"confirmed_at"`
	CreatedAt           time.Time         `json:"created_at"`
	PayURL              string            `json:"pay_url"`
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
	Amount      string            `json:"amount"`
	Currency    string            `json:"currency"`
	OrderID     string            `json:"order_id,omitempty"`
	Description string            `json:"description,omitempty"`
	FeeBearer   string            `json:"fee_bearer,omitempty"`
	ReturnURL   string            `json:"return_url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
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

type EventData struct {
	Object string  `json:"object"`
	Charge Charge  `json:"charge"`
	Refund *Refund `json:"refund"`
}
