package wishop

import (
	"fmt"
	"time"
)

type Error interface {
	error
	StatusCode() int
	Message() string
}

type apiError struct {
	statusCode int
	message    string
}

func (e apiError) StatusCode() int { return e.statusCode }

func (e apiError) Message() string { return e.message }

func (e apiError) Error() string {
	return fmt.Sprintf("wishop: %d %s", e.statusCode, e.message)
}

type ValidationError struct{ apiError }

type AuthenticationError struct{ apiError }

type PermissionError struct{ apiError }

type NotFoundError struct{ apiError }

type ConflictError struct{ apiError }

type ChargeExpiredError struct{ apiError }

type PayloadTooLargeError struct{ apiError }

type UnprocessableError struct{ apiError }

type RateLimitError struct {
	apiError
	RetryAfter time.Duration
}

type APIError struct{ apiError }

type SignatureVerificationError struct {
	Reason string
}

func (e *SignatureVerificationError) Error() string {
	return "wishop: webhook signature verification failed: " + e.Reason
}

func errorFor(status int, message string, retryAfter time.Duration) error {
	base := apiError{statusCode: status, message: message}

	switch status {
	case 400:
		return &ValidationError{base}
	case 401:
		return &AuthenticationError{base}
	case 403:
		return &PermissionError{base}
	case 404:
		return &NotFoundError{base}
	case 409:
		return &ConflictError{base}
	case 410:
		return &ChargeExpiredError{base}
	case 413:
		return &PayloadTooLargeError{base}
	case 422:
		return &UnprocessableError{base}
	case 429:
		return &RateLimitError{apiError: base, RetryAfter: retryAfter}
	default:
		return &APIError{base}
	}
}
