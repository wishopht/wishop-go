package wishop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

const payoutJSON = `{"id":"po_7Hn2vB9xQm4tRkLw","recipient_phone":"+50931234567","recipient_name":"Jean Baptiste","currency":"HTG","amount":"150.00","fee":"3.00","status":"SUCCEEDED","fail_reason":null,"reference":"June payroll","metadata":{"employee_id":4471},"livemode":true,"created_at":"2026-08-31T12:04:11.318Z","completed_at":"2026-08-31T12:04:12.006Z"}`

func TestCreatePayoutSendsIdempotencyKey(t *testing.T) {
	var (
		method string
		path   string
		key    string
		body   map[string]any
	)

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		key = r.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status_code":201,"success":true,"message":"payout created","data":`+payoutJSON+`}`)
	})

	payout, err := client.Payouts.Create(context.Background(), "payroll-2026-06-emp-4471", CreatePayoutParams{
		Amount:         "150.00",
		Currency:       "HTG",
		RecipientPhone: "+50931234567",
		Reference:      "June payroll",
	})
	if err != nil {
		t.Fatalf("create payout: %v", err)
	}

	if method != http.MethodPost || path != "/v1/payouts" {
		t.Errorf("request = %s %s", method, path)
	}
	// Unlike a charge, the key is never optional — the API rejects a payout
	// without one, so the SDK must always send it.
	if key != "payroll-2026-06-emp-4471" {
		t.Errorf("Idempotency-Key = %q, want the key passed to Create", key)
	}
	if body["recipient_phone"] != "+50931234567" || body["amount"] != "150.00" {
		t.Errorf("body = %+v", body)
	}
	if _, sent := body["recipient_name"]; sent {
		t.Error("recipient_name must not be sent: it is resolved from the wallet")
	}
	if payout.Status != PayoutStatusSucceeded || payout.Fee != "3.00" {
		t.Errorf("payout = %+v", payout)
	}
	if payout.CompletedAt == nil {
		t.Error("completed_at was not parsed")
	}
	if payout.Metadata["employee_id"] != float64(4471) {
		t.Errorf("metadata = %+v, want arbitrary JSON values", payout.Metadata)
	}
}

// An empty key is refused before any HTTP happens: the request would be
// rejected anyway, and failing locally keeps the mistake obvious.
func TestCreatePayoutRequiresIdempotencyKey(t *testing.T) {
	called := false
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	_, err := client.Payouts.Create(context.Background(), "", CreatePayoutParams{
		Amount: "150.00", Currency: "HTG", RecipientPhone: "+50931234567",
	})
	if err == nil {
		t.Fatal("want an error for an empty idempotency key")
	}
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Errorf("err = %T, want ValidationError", err)
	}
	if called {
		t.Error("a request was sent despite the missing key")
	}
}

// A 202 is not an error: the payout exists and is resolving. Callers must be
// able to read its PENDING status rather than see a failure.
func TestCreatePayoutAcceptedIsNotAnError(t *testing.T) {
	pending := `{"id":"po_2Bq8mL4xVn6tKdRw","recipient_phone":"+50900000006","recipient_name":null,"currency":"HTG","amount":"150.00","fee":"3.00","status":"PENDING","fail_reason":null,"reference":null,"metadata":null,"livemode":false,"created_at":"2026-08-31T12:09:43.882Z","completed_at":null}`

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"status_code":202,"success":true,"message":"payout accepted","data":`+pending+`}`)
	})

	payout, err := client.Payouts.Create(context.Background(), "key-1", CreatePayoutParams{
		Amount: "150.00", Currency: "HTG", RecipientPhone: "+50900000006",
	})
	if err != nil {
		t.Fatalf("202 surfaced as an error: %v", err)
	}
	if payout.Status != PayoutStatusPending {
		t.Errorf("status = %q, want PENDING", payout.Status)
	}
	if payout.CompletedAt != nil {
		t.Error("a pending payout must not carry completed_at")
	}
}

// A decline is an UnprocessableError, and the payout it recorded is readable.
func TestCreatePayoutDeclineIsUnprocessable(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"status_code":422,"success":false,"message":"payout declined: merchant balance insufficient","data":null}`)
	})

	_, err := client.Payouts.Create(context.Background(), "key-2", CreatePayoutParams{
		Amount: "999999.00", Currency: "HTG", RecipientPhone: "+50900000001",
	})
	var unprocessable *UnprocessableError
	if !errors.As(err, &unprocessable) {
		t.Fatalf("err = %T, want UnprocessableError", err)
	}
	if unprocessable.Message() != "payout declined: merchant balance insufficient" {
		t.Errorf("message = %q", unprocessable.Message())
	}
}

func TestListPayoutsSendsFilters(t *testing.T) {
	var query string

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status_code":200,"success":true,"message":"payouts retrieved","data":{"payouts":[`+payoutJSON+`],"has_more":true}}`)
	})

	list, err := client.Payouts.List(context.Background(), ListPayoutsParams{
		Limit: 25, Status: PayoutStatusFailed, Currency: "HTG", StartingAfter: "po_prev",
	})
	if err != nil {
		t.Fatalf("list payouts: %v", err)
	}

	for _, want := range []string{"limit=25", "status=FAILED", "currency=HTG", "starting_after=po_prev"} {
		if !contains(query, want) {
			t.Errorf("query %q missing %q", query, want)
		}
	}
	if len(list.Payouts) != 1 || !list.HasMore {
		t.Errorf("list = %+v", list)
	}
}

func TestRetrievePayoutEscapesID(t *testing.T) {
	var path string

	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status_code":200,"success":true,"message":"payout retrieved","data":`+payoutJSON+`}`)
	})

	if _, err := client.Payouts.Retrieve(context.Background(), "po_7Hn2vB9xQm4tRkLw"); err != nil {
		t.Fatalf("retrieve payout: %v", err)
	}
	if path != "/v1/payouts/po_7Hn2vB9xQm4tRkLw" {
		t.Errorf("path = %q", path)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
