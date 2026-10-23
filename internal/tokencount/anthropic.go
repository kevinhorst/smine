package tokencount

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	anthropicVersion  = "2023-06-01"
	countTokensURL    = "https://api.anthropic.com/v1/messages/count_tokens"
	errorTypeNotFound = "not_found_error"
	fallbackModel     = "claude-sonnet-5"
	headerApiKey      = "x-api-key"
	headerContentType = "content-type"
	headerRetryAfter  = "retry-after"
	headerVersion     = "anthropic-version"
	maxResponseBytes  = 64 << 10
	maxRetryAfter     = 5 * time.Second
	mimeJson          = "application/json"
	requestTimeout    = 15 * time.Second
	roleUser          = "user"
)

// anthropicClient is the binary's only Anthropic call: one endpoint, one field read back.
type anthropicClient struct {
	apiKey   string
	endpoint string
	http     *http.Client
}

func newAnthropicClient(apiKey, endpoint string) *anthropicClient {
	client := &anthropicClient{
		apiKey:   apiKey,
		endpoint: endpoint,
		http:     &http.Client{Timeout: requestTimeout},
	}
	return client
}

// countTokens retries a 429 once when its retry-after is at most maxRetryAfter.
func (a *anthropicClient) countTokens(ctx context.Context, model, text string) (int, error) {
	tokens, err := a.post(ctx, model, text)
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests || apiErr.retryAfter == 0 {
		return tokens, err
	}

	timer := time.NewTimer(apiErr.retryAfter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("anthropicClient.countTokens: %w", ctx.Err())
	case <-timer.C:
	}
	return a.post(ctx, model, text)
}

func (a *anthropicClient) post(ctx context.Context, model, text string) (int, error) {
	message := countMessage{Content: text, Role: roleUser}
	payload := countRequest{Messages: []countMessage{message}, Model: model}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("anthropicClient.post: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("anthropicClient.post: %w", err)
	}
	request.Header.Set(headerApiKey, a.apiKey)
	request.Header.Set(headerContentType, mimeJson)
	request.Header.Set(headerVersion, anthropicVersion)

	response, err := a.http.Do(request)
	if err != nil {
		return 0, fmt.Errorf("anthropicClient.post: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return 0, fmt.Errorf("anthropicClient.post: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return 0, newApiError(response, data)
	}

	var parsed countResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return 0, fmt.Errorf("anthropicClient.post: Non-JSON count_tokens response: %w", err)
	}
	return parsed.InputTokens, nil
}

// apiError is a non-2xx count_tokens answer; the message never carries the key.
type apiError struct {
	Message    string
	Status     int
	Type       string
	retryAfter time.Duration
}

func newApiError(response *http.Response, data []byte) *apiError {
	var failure errorResponse
	_ = json.Unmarshal(data, &failure)
	apiErr := &apiError{Message: failure.Error.Message, Status: response.StatusCode, Type: failure.Error.Type}
	if seconds, err := strconv.Atoi(response.Header.Get(headerRetryAfter)); err == nil {
		wait := time.Duration(seconds) * time.Second
		if wait > 0 && wait <= maxRetryAfter {
			apiErr.retryAfter = wait
		}
	}
	return apiErr
}

func (e *apiError) Error() string {
	return fmt.Sprintf("anthropicClient.post: HTTP %d %s: %s", e.Status, e.Type, e.Message)
}

type countMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type countRequest struct {
	Messages []countMessage `json:"messages"`
	Model    string         `json:"model"`
}

type countResponse struct {
	InputTokens int `json:"input_tokens"`
}

type errorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}
