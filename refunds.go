package wishop

import (
	"context"
	"net/http"
	"net/url"
)

type RefundService struct {
	client *Client
}

func (s *RefundService) Create(ctx context.Context, chargeID string, params CreateRefundParams, opts ...RequestOption) (*Refund, error) {
	refund := &Refund{}
	path := "/v1/charges/" + url.PathEscape(chargeID) + "/refunds"

	if err := s.client.do(ctx, http.MethodPost, path, nil, params, opts, refund); err != nil {
		return nil, err
	}

	return refund, nil
}

func (s *RefundService) List(ctx context.Context, chargeID string) ([]Refund, error) {
	list := &refundList{}
	path := "/v1/charges/" + url.PathEscape(chargeID) + "/refunds"

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, nil, list); err != nil {
		return nil, err
	}

	return list.Refunds, nil
}
