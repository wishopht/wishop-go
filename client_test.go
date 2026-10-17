package wishop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const chargeJSON = `{"id":"ch_3kQ8xR2mVn7pLdYt","order_id":"order_1042","description":"Two large coffees","currency":"HTG","amount":"1500.00","fee_bearer":"merchant","fee":"45.00","net":"1455.00","customer_debit_amount":"1500.00","amount_refunded":"0.00","customer_phone_masked":null,"channel":"app","status":"INITIATED","fail_reason":null,"return_url":"https://yourstore.com/orders/1042/complete","metadata":{"cart_id":"c_88213"},"livemode":false,"expires_at":"2026-08-12T15:30:00.000Z","confirmed_at":null,"created_at":"2026-08-12T15:00:00.000Z","pay_url":"https://gatewayapi.wishopht.com/pay/9f2c7a1e"}`

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return New("sk_test_key", WithBaseURL(server.URL))
}

func TestCreateChargeSendsRequestAndUnwrapsEnvelope(t *testing.T) {
	var (
		method string
		path   string
		auth   string
		agent  string
		key    string
		body   map[string]any
	)

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		agent = r.Header.Get("User-Agent")
		key = r.Header.Get("Idempotency-Key")

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status_code":201,"success":true,"message":"charge created","data":`+chargeJSON+`}`)
	})

	charge, err := client.Charges.Create(context.Background(), CreateChargeParams{
		Amount:   "1500.00",
		Currency: "HTG",
		OrderID:  "order_1042",
		Metadata: map[string]any{"cart_id": "c_88213"},
	}, WithIdempotencyKey("order-1042-attempt-1"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if method != http.MethodPost || path != "/v1/charges" {
		t.Errorf("requested %s %s", method, path)
	}

	if auth != "Bearer sk_test_key" {
		t.Errorf("authorization = %q", auth)
	}

	if agent != "wishop-go/"+Version {
		t.Errorf("user agent = %q", agent)
	}

	if key != "order-1042-attempt-1" {
		t.Errorf("idempotency key = %q", key)
	}

	if body["amount"] != "1500.00" || body["order_id"] != "order_1042" {
		t.Errorf("request body = %v", body)
	}

	if _, present := body["description"]; present {
		t.Error("expected omitted optional fields to be absent from the body")
	}

	if charge.ID != "ch_3kQ8xR2mVn7pLdYt" || charge.PayURL == "" {
		t.Errorf("charge = %+v", charge)
	}

	if charge.Metadata["cart_id"] != "c_88213" {
		t.Errorf("metadata = %v", charge.Metadata)
	}
}

func TestCreateChargeOmitsIdempotencyKeyByDefault(t *testing.T) {
	var present bool

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]

		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status_code":201,"success":true,"message":"charge created","data":`+chargeJSON+`}`)
	})

	if _, err := client.Charges.Create(context.Background(), CreateChargeParams{Amount: "1500.00", Currency: "HTG"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if present {
		t.Error("expected no idempotency key unless one was supplied")
	}
}

func TestChargeParsesTimestampsAndNullables(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status_code":200,"success":true,"message":"charge retrieved","data":`+chargeJSON+`}`)
	})

	charge, err := client.Charges.Retrieve(context.Background(), "ch_3kQ8xR2mVn7pLdYt")
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}

	expected := time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC)
	if !charge.CreatedAt.Equal(expected) {
		t.Errorf("created at = %v", charge.CreatedAt)
	}

	if !charge.ConfirmedAt.IsZero() {
		t.Errorf("expected a null confirmed_at to be the zero time, got %v", charge.ConfirmedAt)
	}

	if charge.FailReason != "" || charge.CustomerPhoneMasked != "" {
		t.Error("expected null strings to decode as empty")
	}
}

func TestListChargesSerializesQueryParameters(t *testing.T) {
	var query string

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		io.WriteString(w, `{"status_code":200,"success":true,"message":"charges retrieved","data":{"charges":[`+chargeJSON+`],"has_more":true}}`)
	})

	list, err := client.Charges.List(context.Background(), ListChargesParams{
		Limit:         100,
		StartingAfter: "ch_5Tn9wY3pQr8kMxZv",
		Status:        ChargeStatusPaid,
		Query:         "order_1042",
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	expected := "limit=100&q=order_1042&starting_after=ch_5Tn9wY3pQr8kMxZv&status=PAID"
	if query != expected {
		t.Errorf("query = %q", query)
	}

	if !list.HasMore || len(list.Charges) != 1 {
		t.Errorf("list = %+v", list)
	}
}

func TestListChargesOmitsUnsetParameters(t *testing.T) {
	var query string

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		io.WriteString(w, `{"status_code":200,"success":true,"message":"charges retrieved","data":{"charges":[],"has_more":false}}`)
	})

	if _, err := client.Charges.List(context.Background(), ListChargesParams{}); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if query != "" {
		t.Errorf("query = %q", query)
	}
}

func TestCreateRefundPostsToTheCharge(t *testing.T) {
	var path string

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path

		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status_code":201,"success":true,"message":"refund created","data":{"id":"re_8Wq3nK6yTf2mBxLp","charge":"ch_3kQ8xR2mVn7pLdYt","amount":"500.00","currency":"HTG","reason":"requested_by_customer","description":null,"status":"succeeded","livemode":false,"created_at":"2026-08-12T17:22:08.119Z"}}`)
	})

	refund, err := client.Refunds.Create(context.Background(), "ch_3kQ8xR2mVn7pLdYt", CreateRefundParams{
		Amount: "500.00",
		Reason: RefundReasonRequestedByCustomer,
	})
	if err != nil {
		t.Fatalf("refund failed: %v", err)
	}

	if path != "/v1/charges/ch_3kQ8xR2mVn7pLdYt/refunds" {
		t.Errorf("path = %q", path)
	}

	if refund.ID != "re_8Wq3nK6yTf2mBxLp" || refund.Status != RefundStatusSucceeded {
		t.Errorf("refund = %+v", refund)
	}
}

func TestListRefundsReturnsSlice(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status_code":200,"success":true,"message":"refunds retrieved","data":{"refunds":[{"id":"re_8Wq3nK6yTf2mBxLp","charge":"ch_3kQ8xR2mVn7pLdYt","amount":"500.00","currency":"HTG","reason":"other","description":null,"status":"succeeded","livemode":false,"created_at":"2026-08-13T09:05:44.301Z"}]}}`)
	})

	refunds, err := client.Refunds.List(context.Background(), "ch_3kQ8xR2mVn7pLdYt")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(refunds) != 1 {
		t.Fatalf("expected one refund, got %d", len(refunds))
	}
}

func TestStatusCodesMapToTypedErrors(t *testing.T) {
	cases := []struct {
		status int
		target any
	}{
		{400, new(*ValidationError)},
		{401, new(*AuthenticationError)},
		{403, new(*PermissionError)},
		{404, new(*NotFoundError)},
		{409, new(*ConflictError)},
		{410, new(*ChargeExpiredError)},
		{413, new(*PayloadTooLargeError)},
		{422, new(*UnprocessableError)},
		{429, new(*RateLimitError)},
		{500, new(*APIError)},
		{503, new(*APIError)},
	}

	for _, testCase := range cases {
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(testCase.status)
			io.WriteString(w, `{"status_code":0,"success":false,"message":"the gateway said no","data":null}`)
		})

		_, err := client.Charges.Retrieve(context.Background(), "ch_missing")

		if !errors.As(err, testCase.target) {
			t.Errorf("status %d produced %T", testCase.status, err)
			continue
		}

		var wishopErr Error
		if !errors.As(err, &wishopErr) {
			t.Errorf("status %d does not satisfy the Error interface", testCase.status)
			continue
		}

		if wishopErr.StatusCode() != testCase.status {
			t.Errorf("status %d reported %d", testCase.status, wishopErr.StatusCode())
		}

		if wishopErr.Message() != "the gateway said no" {
			t.Errorf("status %d reported message %q", testCase.status, wishopErr.Message())
		}
	}
}

func TestRateLimitErrorCarriesRetryAfter(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "12")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"status_code":429,"success":false,"message":"rate limit exceeded","data":null}`)
	})

	_, err := client.Charges.Retrieve(context.Background(), "ch_3kQ8xR2mVn7pLdYt")

	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected a rate limit error, got %v", err)
	}

	if rateLimitErr.RetryAfter != 12*time.Second {
		t.Errorf("retry after = %v", rateLimitErr.RetryAfter)
	}
}

func TestNonJSONFailureStillMapsToTypedError(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "<html>upstream unavailable</html>")
	})

	_, err := client.Charges.Retrieve(context.Background(), "ch_3kQ8xR2mVn7pLdYt")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error, got %v", err)
	}
}

func TestTimeoutIsApplied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client := New("sk_test_key", WithBaseURL(server.URL), WithTimeout(50*time.Millisecond))

	if _, err := client.Charges.Retrieve(context.Background(), "ch_3kQ8xR2mVn7pLdYt"); err == nil {
		t.Fatal("expected the request to time out")
	}
}

func TestChargeExposesChannelAndNonStringMetadata(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status_code":200,"success":true,"message":"ok","data":{"id":"ch_1","channel":"app","metadata":{"seats":4,"vip":true,"note":"x"}}}`)
	})

	charge, err := client.Charges.Retrieve(context.Background(), "ch_1")
	if err != nil {
		t.Fatalf("a charge with non-string metadata must decode: %v", err)
	}

	if charge.Channel != ChargeChannelApp {
		t.Errorf("channel = %q", charge.Channel)
	}

	if charge.Metadata["seats"] != float64(4) || charge.Metadata["vip"] != true || charge.Metadata["note"] != "x" {
		t.Errorf("metadata = %v", charge.Metadata)
	}
}
