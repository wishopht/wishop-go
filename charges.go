package wishop

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type ChargeService struct {
	client *Client
}

func (s *ChargeService) Create(ctx context.Context, params CreateChargeParams, opts ...RequestOption) (*Charge, error) {
	charge := &Charge{}

	if err := s.client.do(ctx, http.MethodPost, "/v1/charges", nil, params, opts, charge); err != nil {
		return nil, err
	}

	return charge, nil
}

func (s *ChargeService) Retrieve(ctx context.Context, id string) (*Charge, error) {
	charge := &Charge{}

	if err := s.client.do(ctx, http.MethodGet, "/v1/charges/"+url.PathEscape(id), nil, nil, nil, charge); err != nil {
		return nil, err
	}

	return charge, nil
}

func (s *ChargeService) List(ctx context.Context, params ListChargesParams) (*ChargeList, error) {
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

	if params.Query != "" {
		query.Set("q", params.Query)
	}

	list := &ChargeList{}

	if err := s.client.do(ctx, http.MethodGet, "/v1/charges", query, nil, nil, list); err != nil {
		return nil, err
	}

	return list, nil
}
