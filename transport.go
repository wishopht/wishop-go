package wishop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type envelope struct {
	StatusCode int             `json:"status_code"`
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, opts []RequestOption, out any) error {
	options := &requestOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("wishop: encoding request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return fmt.Errorf("wishop: building request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if options.idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", options.idempotencyKey)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wishop: request failed: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("wishop: reading response: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return errorFor(res.StatusCode, http.StatusText(res.StatusCode), retryAfter(res))
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return errorFor(res.StatusCode, env.Message, retryAfter(res))
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("wishop: decoding response: %w", err)
	}

	return nil
}

func retryAfter(res *http.Response) time.Duration {
	seconds, err := strconv.Atoi(res.Header.Get("Retry-After"))
	if err != nil || seconds < 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
