package tokencount

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGuard(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		status int
	}

	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	clockAt := func(current *time.Time) func() time.Time {
		return func() time.Time { return *current }
	}
	serverErr := &apiError{Status: http.StatusInternalServerError}

	t.Run("rate-limit-and-refill", func(t *testing.T) {
		current := syncNow
		limiter := newGuard(clockAt(&current))

		for range RatePerMinute {
			assert.True(t, limiter.take())
		}

		assert.False(t, limiter.take())

		current = current.Add(2 * time.Second)

		assert.True(t, limiter.take())
		assert.False(t, limiter.take())
	})

	t.Run("three-server-errors-trip", func(t *testing.T) {
		breaker := newGuard(clockAt(&syncNow))

		assert.False(t, breaker.recordFailure(serverErr))
		assert.False(t, breaker.recordFailure(serverErr))
		assert.True(t, breaker.recordFailure(serverErr))
		assert.True(t, breaker.isTripped())
	})

	t.Run("success-resets-count", func(t *testing.T) {
		breaker := newGuard(clockAt(&syncNow))

		assert.False(t, breaker.recordFailure(serverErr))
		assert.False(t, breaker.recordFailure(serverErr))
		breaker.recordSuccess()
		assert.False(t, breaker.recordFailure(serverErr))
		assert.False(t, breaker.recordFailure(serverErr))
		assert.False(t, breaker.isTripped())
	})

	t.Run("bad-request-never-trips", func(t *testing.T) {
		breaker := newGuard(clockAt(&syncNow))
		badRequestErr := &apiError{Status: http.StatusBadRequest}

		for range breakerTrips + 1 {
			assert.False(t, breaker.recordFailure(badRequestErr))
		}

		assert.False(t, breaker.isTripped())
	})

	t.Run("non-api-error-never-trips", func(t *testing.T) {
		breaker := newGuard(clockAt(&syncNow))
		plainErr := errors.New("TestGuard: Connection refused")

		for range breakerTrips + 1 {
			assert.False(t, breaker.recordFailure(plainErr))
		}

		assert.False(t, breaker.isTripped())
	})

	tests := make([]*testCase, 0)

	// bad-request
	tests = append(tests, &testCase{
		_id:       "bad-request",
		_expected: false,

		status: http.StatusBadRequest,
	})

	// unauthorized
	tests = append(tests, &testCase{
		_id:       "unauthorized",
		_expected: true,

		status: http.StatusUnauthorized,
	})

	// forbidden
	tests = append(tests, &testCase{
		_id:       "forbidden",
		_expected: true,

		status: http.StatusForbidden,
	})

	// not-found
	tests = append(tests, &testCase{
		_id:       "not-found",
		_expected: false,

		status: http.StatusNotFound,
	})

	// too-many-requests
	tests = append(tests, &testCase{
		_id:       "too-many-requests",
		_expected: true,

		status: http.StatusTooManyRequests,
	})

	// internal-server-error
	tests = append(tests, &testCase{
		_id:       "internal-server-error",
		_expected: true,

		status: http.StatusInternalServerError,
	})

	// service-unavailable
	tests = append(tests, &testCase{
		_id:       "service-unavailable",
		_expected: true,

		status: http.StatusServiceUnavailable,
	})

	// Run tests
	for _, test := range tests {
		t.Run("trips-breaker-"+test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, tripsBreaker(test.status))
		})
	}
}
