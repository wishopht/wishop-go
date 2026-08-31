package wishop

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type PayoutService struct {
	client *Client
}

// Create sends money from your wallet to a customer's.
//
// idempotencyKey is a required parameter rather than an option because the API
// requires it: a payout pushes money out, so a retry after a dropped connection
// is indistinguishable from a second, deliberate payment. Derive it from the
// thing being paid for and reuse it on every retry of that payout — a key
// generated per attempt defeats the mechanism entirely.
//
// Three outcomes, and the third is the one to handle deliberately:
//
//   - the payout comes back PayoutStatusSucceeded — the money moved;
//   - it comes back PayoutStatusPending — the wallet's answer was lost, so the
//     money may or may not have moved. It resolves on its own and a webhook
//     reports the outcome. Never send a replacement; retry with this same key
//     if you must, which returns the same payout;
//   - the call returns an error — an UnprocessableError means declined, and a
//     FAILED payout is still recorded, retrievable with Retrieve.
func (s *PayoutService) Create(
	ctx context.Context, idempotencyKey string, params CreatePayoutParams, opts ...RequestOption,
) (*Payout, error) {
	if idempotencyKey == "" {
		return nil, &ValidationError{apiError{
			statusCode: http.StatusBadRequest,
			message:    "an idempotency key is required for payouts",
		}}
	}

	payout := &Payout{}
	opts = append(opts, WithIdempotencyKey(idempotencyKey))

	if err := s.client.do(ctx, http.MethodPost, "/v1/payouts", nil, params, opts, payout); err != nil {
		return nil, err
	}

	return payout, nil
}

// Retrieve reads one payout. This is how you check on a payout that came back
// pending, and how you read one that was declined.
func (s *PayoutService) Retrieve(ctx context.Context, id string) (*Payout, error) {
	payout := &Payout{}

	if err := s.client.do(ctx, http.MethodGet, "/v1/payouts/"+url.PathEscape(id), nil, nil, nil, payout); err != nil {
		return nil, err
	}

	return payout, nil
}

func (s *PayoutService) List(ctx context.Context, params ListPayoutsParams) (*PayoutList, error) {
	query := url.Values{}

	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.StartingAfter != "" {
		query.Set("starting_after", params.StartingAfter)
	}

	if params.Status != "" {
		query.Set("status", params.Status)
	}

	if params.Currency != "" {
		query.Set("currency", params.Currency)
	}

	list := &PayoutList{}

	if err := s.client.do(ctx, http.MethodGet, "/v1/payouts", query, nil, nil, list); err != nil {
		return nil, err
	}

	return list, nil
}
