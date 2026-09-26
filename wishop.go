package wishop

import (
	"net/http"
	"strings"
	"time"
)

const (
	Version        = "1.1.0"
	defaultBaseURL = "https://gatewayapi.wishopht.com"
	defaultTimeout = 30 * time.Second
)

type Client struct {
	Charges *ChargeService
	Refunds *RefundService
	Payouts *PayoutService

	apiKey     string
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
	userAgent  string
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) { c.timeout = timeout }
}

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		timeout:    defaultTimeout,
		httpClient: http.DefaultClient,
		userAgent:  "wishop-go/" + Version,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.Charges = &ChargeService{client: c}
	c.Refunds = &RefundService{client: c}
	c.Payouts = &PayoutService{client: c}

	return c
}

type requestOptions struct {
	idempotencyKey string
}

type RequestOption func(*requestOptions)

func WithIdempotencyKey(key string) RequestOption {
	return func(o *requestOptions) { o.idempotencyKey = key }
}
