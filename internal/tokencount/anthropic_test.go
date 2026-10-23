package tokencount

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testApiKey      = "test-key"
	testCountPath   = "/v1/messages/count_tokens"
	testRequestBody = `{"messages":[{"content":"hello","role":"user"}],"model":"claude-sonnet-5"}`
	testUnauthBody  = `{"type":"error","error":{"type":"authentication_error","message":"x-api-key header is required"},"request_id":"req_1"}`
)

type cannedResponse struct {
	body       string
	retryAfter string
	status     int
}

// countTokensServer serves its canned responses in order (the last one repeats)
// and records every request it receives.
type countTokensServer struct {
	mutex     sync.Mutex
	requests  []*recordedRequest
	responses []*cannedResponse
	server    *httptest.Server
}

func newCountTokensServer(t *testing.T, responses ...*cannedResponse) *countTokensServer {
	recorder := &countTokensServer{
		requests:  make([]*recordedRequest, 0),
		responses: responses,
	}
	recorder.server = httptest.NewServer(http.HandlerFunc(recorder.serve))
	t.Cleanup(recorder.server.Close)
	return recorder
}

func (s *countTokensServer) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	request := &recordedRequest{
		apiKey:      r.Header.Get(headerApiKey),
		body:        string(body),
		contentType: r.Header.Get(headerContentType),
		method:      r.Method,
		path:        r.URL.Path,
		version:     r.Header.Get(headerVersion),
	}

	s.mutex.Lock()
	s.requests = append(s.requests, request)
	response := s.responses[min(len(s.requests), len(s.responses))-1]
	s.mutex.Unlock()

	if response.retryAfter != "" {
		w.Header().Set(headerRetryAfter, response.retryAfter)
	}
	w.Header().Set(headerContentType, mimeJson)
	w.WriteHeader(response.status)
	_, _ = io.WriteString(w, response.body)
}

func (s *countTokensServer) client() *anthropicClient {
	return newAnthropicClient(testApiKey, s.server.URL+testCountPath)
}

func (s *countTokensServer) recorded() []*recordedRequest {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return append(make([]*recordedRequest, 0, len(s.requests)), s.requests...)
}

type recordedRequest struct {
	apiKey      string
	body        string
	contentType string
	method      string
	path        string
	version     string
}

func apiErrorOf(err error) *apiError {
	if apiErr, ok := errors.AsType[*apiError](err); ok {
		return apiErr
	}
	return &apiError{}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func expectedRequests(count int) []*recordedRequest {
	requests := make([]*recordedRequest, 0, count)
	for range count {
		request := &recordedRequest{
			apiKey:      testApiKey,
			body:        testRequestBody,
			contentType: mimeJson,
			method:      http.MethodPost,
			path:        testCountPath,
			version:     anthropicVersion,
		}
		requests = append(requests, request)
	}
	return requests
}

func TestAnthropicClient_CountTokens(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCalls  int
		_expectedStatus int
		_expectedTokens int
		_expectedType   string
		_shouldPass     bool

		ctx    context.Context
		server *countTokensServer
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := make([]*testCase, 0)

	// ok
	tests = append(tests, &testCase{
		_id:             "ok",
		_expectedCalls:  1,
		_expectedStatus: 0,
		_expectedTokens: 12,
		_expectedType:   "",
		_shouldPass:     true,

		ctx:    context.Background(),
		server: newCountTokensServer(t, &cannedResponse{body: `{"input_tokens":12}`, retryAfter: "", status: http.StatusOK}),
	})

	// unauthenticated
	tests = append(tests, &testCase{
		_id:             "unauthenticated",
		_expectedCalls:  1,
		_expectedStatus: http.StatusUnauthorized,
		_expectedTokens: 0,
		_expectedType:   "authentication_error",
		_shouldPass:     false,

		ctx:    context.Background(),
		server: newCountTokensServer(t, &cannedResponse{body: testUnauthBody, retryAfter: "", status: http.StatusUnauthorized}),
	})

	// model-not-found
	notFound := &cannedResponse{
		body:       `{"type":"error","error":{"type":"not_found_error","message":"model: claude-x"}}`,
		retryAfter: "",
		status:     http.StatusNotFound,
	}
	tests = append(tests, &testCase{
		_id:             "model-not-found",
		_expectedCalls:  1,
		_expectedStatus: http.StatusNotFound,
		_expectedTokens: 0,
		_expectedType:   errorTypeNotFound,
		_shouldPass:     false,

		ctx:    context.Background(),
		server: newCountTokensServer(t, notFound),
	})

	// rate-limited-short-retry-after
	shortRateLimit := &cannedResponse{
		body:       `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`,
		retryAfter: "1",
		status:     http.StatusTooManyRequests,
	}
	tests = append(tests, &testCase{
		_id:             "rate-limited-short-retry-after",
		_expectedCalls:  2,
		_expectedStatus: 0,
		_expectedTokens: 7,
		_expectedType:   "",
		_shouldPass:     true,

		ctx:    context.Background(),
		server: newCountTokensServer(t, shortRateLimit, &cannedResponse{body: `{"input_tokens":7}`, retryAfter: "", status: http.StatusOK}),
	})

	// rate-limited-long-retry-after
	longRateLimit := &cannedResponse{
		body:       `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`,
		retryAfter: "60",
		status:     http.StatusTooManyRequests,
	}
	tests = append(tests, &testCase{
		_id:             "rate-limited-long-retry-after",
		_expectedCalls:  1,
		_expectedStatus: http.StatusTooManyRequests,
		_expectedTokens: 0,
		_expectedType:   "rate_limit_error",
		_shouldPass:     false,

		ctx:    context.Background(),
		server: newCountTokensServer(t, longRateLimit),
	})

	// non-json-ok
	tests = append(tests, &testCase{
		_id:             "non-json-ok",
		_expectedCalls:  1,
		_expectedStatus: 0,
		_expectedTokens: 0,
		_expectedType:   "",
		_shouldPass:     false,

		ctx:    context.Background(),
		server: newCountTokensServer(t, &cannedResponse{body: "<html>gateway</html>", retryAfter: "", status: http.StatusOK}),
	})

	// oversized-body-truncated
	oversized := &cannedResponse{
		body:       `{"input_tokens":3,"padding":"` + strings.Repeat("x", 2*maxResponseBytes) + `"}`,
		retryAfter: "",
		status:     http.StatusOK,
	}
	tests = append(tests, &testCase{
		_id:             "oversized-body-truncated",
		_expectedCalls:  1,
		_expectedStatus: 0,
		_expectedTokens: 0,
		_expectedType:   "",
		_shouldPass:     false,

		ctx:    context.Background(),
		server: newCountTokensServer(t, oversized),
	})

	// context-canceled
	tests = append(tests, &testCase{
		_id:             "context-canceled",
		_expectedCalls:  0,
		_expectedStatus: 0,
		_expectedTokens: 0,
		_expectedType:   "",
		_shouldPass:     false,

		ctx:    canceled,
		server: newCountTokensServer(t, &cannedResponse{body: `{"input_tokens":12}`, retryAfter: "", status: http.StatusOK}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			tokens, err := test.server.client().countTokens(test.ctx, fallbackModel, "hello")

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedTokens, tokens)
			assert.Equal(t, test._expectedStatus, apiErrorOf(err).Status)
			assert.Equal(t, test._expectedType, apiErrorOf(err).Type)
			assert.NotContains(t, errorText(err), testApiKey)
			assert.Equal(t, expectedRequests(test._expectedCalls), test.server.recorded())
		})
	}
}

func TestCountTokensUrl(t *testing.T) {
	assert.Equal(t, "https://api.anthropic.com/v1/messages/count_tokens", countTokensURL)
}
