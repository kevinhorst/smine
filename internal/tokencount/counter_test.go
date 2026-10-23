package tokencount

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

// testApiKey and testCountPath are shared with anthropic_test.go.
const (
	notFoundBody    = `{"type":"error","error":{"type":"not_found_error","message":"model: claude-x"}}`
	serverErrorBody = `{"type":"error","error":{"type":"api_error","message":"Internal server error"}}`
)

// countServer is a count_tokens stub: respond answers each request from its
// decoded body and the 1-based call number; every request is recorded.
type countServer struct {
	mu sync.Mutex

	apiKeys []string
	calls   int
	models  []string
	paths   []string
	respond func(request *countRequest, call int) (int, string)
	server  *httptest.Server
}

func newCountServer(t *testing.T, respond func(request *countRequest, call int) (int, string)) *countServer {
	stub := &countServer{respond: respond}
	stub.server = httptest.NewServer(http.HandlerFunc(stub.serveHTTP))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *countServer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// recorded returns copies of the recorded api keys, models and paths.
func (s *countServer) recorded() ([]string, []string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.apiKeys), slices.Clone(s.models), slices.Clone(s.paths)
}

func (s *countServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	var request countRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.calls++
	call := s.calls
	s.apiKeys = append(s.apiKeys, r.Header.Get(headerApiKey))
	s.models = append(s.models, request.Model)
	s.paths = append(s.paths, r.URL.Path)
	s.mu.Unlock()

	status, body := s.respond(&request, call)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// claudeRecord builds a Claude record with one root agent of model holding
// one hook item per text.
func claudeRecord(model string, texts ...string) *contextrecord.Record {
	items := make([]contextrecord.Item, 0, len(texts))
	for index, text := range texts {
		label := fmt.Sprintf("item-%d", index)
		items = append(items, contextrecord.NewItem(contextrecord.KindHookContext, label, contextrecord.SourceHookContext, text))
	}
	root := contextrecord.Agent{Id: "root", Items: items, Kind: contextrecord.AgentRoot, Model: model}
	record := &contextrecord.Record{Agent: contextrecord.SessionClaude, Agents: []contextrecord.Agent{root}}
	return record
}

// distinctTexts returns count distinct short texts so no two share a cache key.
func distinctTexts(count int) []string {
	texts := make([]string, 0, count)
	for index := range count {
		texts = append(texts, fmt.Sprintf("text %d", index))
	}
	return texts
}

// itemsWith counts the record's items matching predicate.
func itemsWith(record *contextrecord.Record, predicate func(item *contextrecord.Item) bool) int {
	matches := 0
	for agentIndex := range record.Agents {
		for itemIndex := range record.Agents[agentIndex].Items {
			if predicate(&record.Agents[agentIndex].Items[itemIndex]) {
				matches++
			}
		}
	}
	return matches
}

// newTestCounter builds a keychain-mode Counter whose client posts to stub.
func newTestCounter(t *testing.T, stub *countServer) *Counter {
	options := &Options{CacheDir: t.TempDir(), Mode: ModeKeychain}
	counter, err := New(options)
	require.NoError(t, err)

	counter.mode = ModeKeychain
	counter.notice = ""
	counter.anthropic = newAnthropicClient(testApiKey, stub.server.URL+testCountPath)
	return counter
}

// respondTokens answers 200 with the content's byte length as input_tokens.
func respondTokens(request *countRequest, _ int) (int, string) {
	return http.StatusOK, fmt.Sprintf(`{"input_tokens":%d}`, len(request.Messages[0].Content))
}

func TestCounter_CountText(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCalls  int
		_expectedError  string
		_expectedModels []string
		_expectedTokens int

		counter *Counter
		stub    *countServer
		text    string
	}

	ctx := context.Background()
	tests := make([]*testCase, 0)

	// counted
	stub := newCountServer(t, respondTokens)
	tests = append(tests, &testCase{
		_id:             "counted",
		_expectedCalls:  1,
		_expectedError:  "",
		_expectedModels: []string{SkillCountModel},
		_expectedTokens: 10,

		counter: newTestCounter(t, stub),
		stub:    stub,
		text:    "beta gamma",
	})

	// cached-second-call-no-http
	stub = newCountServer(t, respondTokens)
	cachedCounter := newTestCounter(t, stub)
	_, err := cachedCounter.CountText(ctx, SkillCountModel, "alpha")
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:             "cached-second-call-no-http",
		_expectedCalls:  1,
		_expectedError:  "",
		_expectedModels: []string{SkillCountModel},
		_expectedTokens: 5,

		counter: cachedCounter,
		stub:    stub,
		text:    "alpha",
	})

	// mode-off
	offCounter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeOff})
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:             "mode-off",
		_expectedCalls:  0,
		_expectedError:  "Counter.CountText: " + noticeOff,
		_expectedModels: nil,
		_expectedTokens: 0,

		counter: offCounter,
		stub:    newCountServer(t, respondTokens),
		text:    "alpha",
	})

	// key-missing
	keyMissingCounter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeKeychain})
	require.NoError(t, err)
	keyMissingCounter.mode = ModeKeychain
	keyMissingCounter.readKey = func(_ context.Context) (string, error) {
		return "", errors.New("readKeychainKey: No keychain item")
	}
	tests = append(tests, &testCase{
		_id:             "key-missing",
		_expectedCalls:  0,
		_expectedError:  "Counter.CountText: Claude token counting disabled: readKeychainKey: No keychain item",
		_expectedModels: nil,
		_expectedTokens: 0,

		counter: keyMissingCounter,
		stub:    newCountServer(t, respondTokens),
		text:    "alpha",
	})

	// breaker-tripped
	stub = newCountServer(t, respondTokens)
	trippedCounter := newTestCounter(t, stub)
	serverErr := &apiError{Status: http.StatusInternalServerError}
	for range breakerTrips {
		trippedCounter.guard.recordFailure(serverErr)
	}
	tests = append(tests, &testCase{
		_id:             "breaker-tripped",
		_expectedCalls:  0,
		_expectedError:  "Counter.CountText: Not counted (disabled)",
		_expectedModels: nil,
		_expectedTokens: 0,

		counter: trippedCounter,
		stub:    stub,
		text:    "alpha",
	})

	// api-server-error
	respondFailure := func(_ *countRequest, _ int) (int, string) {
		return http.StatusInternalServerError, serverErrorBody
	}
	stub = newCountServer(t, respondFailure)
	tests = append(tests, &testCase{
		_id:             "api-server-error",
		_expectedCalls:  1,
		_expectedError:  "Counter.CountText: anthropicClient.post: HTTP 500 api_error: Internal server error",
		_expectedModels: []string{SkillCountModel},
		_expectedTokens: 0,

		counter: newTestCounter(t, stub),
		stub:    stub,
		text:    "alpha",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			tokens, err := test.counter.CountText(ctx, SkillCountModel, test.text)

			_, models, _ := test.stub.recorded()
			assert.Equal(t, test._expectedTokens, tokens)
			assert.Equal(t, test._expectedError, errorText(err))
			assert.Equal(t, test._expectedCalls, test.stub.callCount())
			assert.Equal(t, test._expectedModels, models)
		})
	}
}

func TestCounter_CountCached(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCalls     int
		_expectedQueued    int
		_expectedTokens    int
		_expectedUncounted []string

		counter *Counter
		record  *contextrecord.Record
		stub    *countServer
	}

	ctx := context.Background()
	oversized := strings.Repeat("x", maxItemRunes+1)
	tests := make([]*testCase, 0)

	// nothing-cached-queues-all
	stub := newCountServer(t, respondTokens)
	tests = append(tests, &testCase{
		_id:                "nothing-cached-queues-all",
		_expectedCalls:     0,
		_expectedQueued:    2,
		_expectedTokens:    0,
		_expectedUncounted: []string{contextrecord.UncountedQueued, contextrecord.UncountedQueued},

		counter: newTestCounter(t, stub),
		record:  claudeRecord("claude-opus-5-5", "alpha", "beta gamma"),
		stub:    stub,
	})

	// cached-filled-rest-queued-oversized-skipped
	stub = newCountServer(t, respondTokens)
	warmCounter := newTestCounter(t, stub)
	warmCounter.Count(ctx, claudeRecord("claude-opus-5-5", "alpha"))
	tests = append(tests, &testCase{
		_id:                "cached-filled-rest-queued-oversized-skipped",
		_expectedCalls:     1,
		_expectedQueued:    1,
		_expectedTokens:    5,
		_expectedUncounted: []string{"", contextrecord.UncountedQueued, contextrecord.UncountedSize},

		counter: warmCounter,
		record:  claudeRecord("claude-opus-5-5", "alpha", "beta gamma", oversized),
		stub:    stub,
	})

	// mode-off-disabled
	offCounter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeOff})
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                "mode-off-disabled",
		_expectedCalls:     0,
		_expectedQueued:    0,
		_expectedTokens:    0,
		_expectedUncounted: []string{contextrecord.UncountedDisabled},

		counter: offCounter,
		record:  claudeRecord("claude-opus-5-5", "alpha"),
		stub:    newCountServer(t, respondTokens),
	})

	// breaker-open-disabled
	stub = newCountServer(t, respondTokens)
	trippedCounter := newTestCounter(t, stub)
	serverErr := &apiError{Status: http.StatusInternalServerError}
	for range breakerTrips {
		trippedCounter.guard.recordFailure(serverErr)
	}
	tests = append(tests, &testCase{
		_id:                "breaker-open-disabled",
		_expectedCalls:     0,
		_expectedQueued:    0,
		_expectedTokens:    0,
		_expectedUncounted: []string{contextrecord.UncountedDisabled},

		counter: trippedCounter,
		record:  claudeRecord("claude-opus-5-5", "alpha"),
		stub:    stub,
	})

	// codex-counted-offline
	codexRoot := contextrecord.Agent{
		Id:    "main",
		Items: []contextrecord.Item{contextrecord.NewItem(contextrecord.KindCodexInstructions, "AGENTS.md", contextrecord.SourceTurn, "hello world")},
		Kind:  contextrecord.AgentRoot,
		Model: "gpt-5.5",
	}
	stub = newCountServer(t, respondTokens)
	tests = append(tests, &testCase{
		_id:                "codex-counted-offline",
		_expectedCalls:     0,
		_expectedQueued:    0,
		_expectedTokens:    2,
		_expectedUncounted: []string{""},

		counter: newTestCounter(t, stub),
		record:  &contextrecord.Record{Agent: contextrecord.SessionCodex, Agents: []contextrecord.Agent{codexRoot}},
		stub:    stub,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			queued := test.counter.CountCached(ctx, test.record)

			uncounted := make([]string, 0, len(test.record.Agents[0].Items))
			for _, item := range test.record.Agents[0].Items {
				uncounted = append(uncounted, item.Uncounted)
			}
			assert.Equal(t, test._expectedQueued, queued)
			assert.Equal(t, test._expectedUncounted, uncounted)
			assert.Equal(t, test._expectedTokens, test.record.Totals.Tokens)
			assert.Equal(t, test._expectedCalls, test.stub.callCount())
		})
	}
}

func TestCounter_Count_Twins(t *testing.T) {
	stub := newCountServer(t, respondTokens)
	counter := newTestCounter(t, stub)
	record := claudeRecord("claude-opus-5-5", "alpha", "beta gamma", "alpha", "alpha")

	counter.Count(context.Background(), record)

	items := record.Agents[0].Items
	assert.Equal(t, 2, stub.callCount())
	assert.Equal(t, []int{5, 10, 5, 5}, []int{items[0].Tokens, items[1].Tokens, items[2].Tokens, items[3].Tokens})
	assert.Equal(t, 0, record.Totals.Uncounted)
	assert.Equal(t, 25, record.Totals.Tokens)
}

func TestCounter_Count_Breaker(t *testing.T) {
	ctx := context.Background()

	t.Run("three-failures-trip", func(t *testing.T) {
		respondFailure := func(_ *countRequest, _ int) (int, string) {
			return http.StatusInternalServerError, serverErrorBody
		}
		stub := newCountServer(t, respondFailure)
		counter := newTestCounter(t, stub)
		record := claudeRecord("claude-opus-5-5", distinctTexts(20)...)

		counter.Count(ctx, record)

		errored := itemsWith(record, func(item *contextrecord.Item) bool { return item.Error != "" })
		disabled := itemsWith(record, func(item *contextrecord.Item) bool { return item.Uncounted == contextrecord.UncountedDisabled })
		assert.Contains(t, counter.Notice(), "paused after 3 consecutive API failures")
		assert.GreaterOrEqual(t, errored, breakerTrips)
		assert.LessOrEqual(t, errored, breakerTrips+poolWorkers-1)
		assert.Equal(t, 20, errored+disabled)
		assert.Equal(t, errored, stub.callCount())
	})

	t.Run("success-resets", func(t *testing.T) {
		respondSequence := func(request *countRequest, call int) (int, string) {
			if call == 2 {
				return respondTokens(request, call)
			}
			return http.StatusInternalServerError, serverErrorBody
		}
		stub := newCountServer(t, respondSequence)
		counter := newTestCounter(t, stub)
		records := make([]*contextrecord.Record, 0, 6)
		for index := range 6 {
			records = append(records, claudeRecord("claude-opus-5-5", fmt.Sprintf("text %d", index)))
		}

		for _, record := range records[:4] {
			counter.Count(ctx, record)
		}
		noticeAfterReset := counter.Notice()
		counter.Count(ctx, records[4])
		noticeAfterTrip := counter.Notice()
		counter.Count(ctx, records[5])

		assert.Empty(t, noticeAfterReset)
		assert.Equal(t, contextrecord.CounterAnthropic, records[1].Agents[0].Items[0].Counter)
		assert.Contains(t, noticeAfterTrip, "paused after 3 consecutive API failures")
		assert.NotEmpty(t, records[4].Agents[0].Items[0].Error)
		assert.Equal(t, contextrecord.UncountedDisabled, records[5].Agents[0].Items[0].Uncounted)
		assert.Equal(t, 5, stub.callCount())
	})
}

func TestCounter_Count_Caps(t *testing.T) {
	stub := newCountServer(t, respondTokens)
	counter := newTestCounter(t, stub)
	clock := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	counter.guard = newGuard(func() time.Time {
		clock = clock.Add(time.Minute)
		return clock
	})
	oversized := strings.Repeat("x", maxItemRunes+1)
	texts := append([]string{oversized}, distinctTexts(MaxItemsPerReport+1)...)
	record := claudeRecord("claude-opus-5-5", texts...)

	counter.Count(context.Background(), record)

	items := record.Agents[0].Items
	counted := itemsWith(record, func(item *contextrecord.Item) bool { return item.Counter == contextrecord.CounterAnthropic })
	assert.Equal(t, contextrecord.UncountedSize, items[0].Uncounted)
	assert.Equal(t, contextrecord.UncountedCap, items[len(items)-1].Uncounted)
	assert.Equal(t, MaxItemsPerReport, counted)
	assert.Equal(t, MaxItemsPerReport, stub.callCount())
	assert.Equal(t, 2, record.Totals.Uncounted)
}

func TestCounter_Count_Claude(t *testing.T) {
	ctx := context.Background()
	stub := newCountServer(t, respondTokens)
	counter := newTestCounter(t, stub)
	first := claudeRecord("claude-opus-5-5[1m]", "alpha", "beta gamma")
	second := claudeRecord("claude-opus-5-5[1m]", "alpha", "beta gamma")

	counter.Count(ctx, first)
	callsAfterFirst := stub.callCount()
	counter.Count(ctx, second)

	apiKeys, models, paths := stub.recorded()
	for _, record := range []*contextrecord.Record{first, second} {
		assert.Equal(t, 5, record.Agents[0].Items[0].Tokens)
		assert.Equal(t, 10, record.Agents[0].Items[1].Tokens)
		assert.Equal(t, contextrecord.CounterAnthropic, record.Agents[0].Items[0].Counter)
		assert.False(t, record.Agents[0].Items[0].Approx)
		assert.Equal(t, 15, record.Totals.Tokens)
		assert.Equal(t, 15, record.Agents[0].Totals.Tokens)
		assert.Equal(t, 0, record.Totals.Uncounted)
	}
	assert.Equal(t, 2, callsAfterFirst)
	assert.Equal(t, 2, stub.callCount())
	assert.Equal(t, []string{"claude-opus-5-5", "claude-opus-5-5"}, models)
	assert.Equal(t, []string{testCountPath, testCountPath}, paths)
	assert.Equal(t, []string{testApiKey, testApiKey}, apiKeys)
	assert.Empty(t, counter.Notice())
}

func TestCounter_Count_ClaudeKeyMissing(t *testing.T) {
	if !KeychainSupported {
		t.Skip("keychain mode degrades to the unsupported notice before any key read")
	}

	counter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeKeychain})
	require.NoError(t, err)

	keyReads := 0
	counter.readKey = func(_ context.Context) (string, error) {
		keyReads++
		return "", errors.New("readKeychainKey: No keychain item")
	}
	first := claudeRecord("claude-opus-5-5", "alpha")
	second := claudeRecord("claude-opus-5-5", "beta")

	counter.Count(context.Background(), first)
	counter.Count(context.Background(), second)

	assert.Equal(t, "Claude token counting disabled: readKeychainKey: No keychain item", counter.Notice())
	assert.Equal(t, 2, keyReads)
	assert.Nil(t, counter.anthropic)
	assert.Equal(t, contextrecord.UncountedDisabled, first.Agents[0].Items[0].Uncounted)
	assert.Equal(t, contextrecord.UncountedDisabled, second.Agents[0].Items[0].Uncounted)
}

func TestCounter_EnsureClient_KeyAddedLater(t *testing.T) {
	if !KeychainSupported {
		t.Skip("keychain mode degrades to the unsupported notice before any key read")
	}

	counter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeKeychain})
	require.NoError(t, err)

	keyReads := 0
	counter.readKey = func(_ context.Context) (string, error) {
		keyReads++
		if keyReads == 1 {
			return "", errors.New("readKeychainKey: No keychain item")
		}
		return testApiKey, nil
	}

	isReadyBefore := counter.ensureClient(context.Background())
	noticeBefore := counter.Notice()
	isReadyAfter := counter.ensureClient(context.Background())
	isReadyAgain := counter.ensureClient(context.Background())

	assert.False(t, isReadyBefore)
	assert.Equal(t, "Claude token counting disabled: readKeychainKey: No keychain item", noticeBefore)
	assert.True(t, isReadyAfter)
	assert.True(t, isReadyAgain)
	assert.Empty(t, counter.Notice())
	assert.NotNil(t, counter.anthropic)
	assert.Equal(t, 2, keyReads)
}

func TestCounter_Count_ClaudeOff(t *testing.T) {
	counter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeOff})
	require.NoError(t, err)

	record := claudeRecord("claude-opus-5-5", "alpha", "beta")

	counter.Count(context.Background(), record)

	disabled := itemsWith(record, func(item *contextrecord.Item) bool { return item.Uncounted == contextrecord.UncountedDisabled })
	assert.Equal(t, 2, disabled)
	assert.Equal(t, 2, record.Totals.Uncounted)
	assert.Equal(t, noticeOff, counter.Notice())
	assert.Nil(t, counter.anthropic)
}

func TestCounter_Count_Codex(t *testing.T) {
	counter, err := New(&Options{CacheDir: t.TempDir(), Mode: ModeOff})
	require.NoError(t, err)

	precounted := contextrecord.NewItem(contextrecord.KindCodexEnvironment, "env", contextrecord.SourceTurn, "hello world")
	precounted.SetTokens(contextrecord.CounterO200k, 99, false)
	exact := contextrecord.Agent{
		Id:    "main",
		Items: []contextrecord.Item{contextrecord.NewItem(contextrecord.KindCodexInstructions, "instructions", contextrecord.SourceTurn, "hello world"), precounted},
		Kind:  contextrecord.AgentRoot,
		Model: "gpt-5.5",
	}
	approx := contextrecord.Agent{
		Id:    "astra",
		Items: []contextrecord.Item{contextrecord.NewItem(contextrecord.KindCodexInstructions, "instructions", contextrecord.SourceTurn, "hello world")},
		Kind:  contextrecord.AgentRoot,
		Model: "gpt-6-astra",
	}
	record := &contextrecord.Record{Agent: contextrecord.SessionCodex, Agents: []contextrecord.Agent{exact, approx}}

	counter.Count(context.Background(), record)

	exactItem := record.Agents[0].Items[0]
	untouchedItem := record.Agents[0].Items[1]
	approxItem := record.Agents[1].Items[0]
	assert.Equal(t, contextrecord.CounterO200k, exactItem.Counter)
	assert.Equal(t, 2, exactItem.Tokens)
	assert.False(t, exactItem.Approx)
	assert.Equal(t, 99, untouchedItem.Tokens)
	assert.Equal(t, contextrecord.CounterO200k, approxItem.Counter)
	assert.Equal(t, 2, approxItem.Tokens)
	assert.True(t, approxItem.Approx)
	assert.Equal(t, 103, record.Totals.Tokens)
}

func TestCounter_Count_Rate(t *testing.T) {
	stub := newCountServer(t, respondTokens)
	counter := newTestCounter(t, stub)
	frozen := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	counter.guard = newGuard(func() time.Time { return frozen })
	record := claudeRecord("claude-opus-5-5", distinctTexts(RatePerMinute+1)...)

	counter.Count(context.Background(), record)

	counted := itemsWith(record, func(item *contextrecord.Item) bool { return item.Counter == contextrecord.CounterAnthropic })
	limited := itemsWith(record, func(item *contextrecord.Item) bool { return item.Uncounted == contextrecord.UncountedRate })
	assert.Equal(t, RatePerMinute, counted)
	assert.Equal(t, 1, limited)
	assert.Equal(t, RatePerMinute, stub.callCount())
}

func TestCounter_Count_UnknownModel(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedApprox  bool
		_expectedCalls   int
		_expectedCounter contextrecord.Counter
		_expectedModels  []string
		_expectedTokens  int
		_hasError        bool

		counter *Counter
		record  *contextrecord.Record
		stub    *countServer
	}

	tests := make([]*testCase, 0)

	// fallback-counts
	respondFallback := func(request *countRequest, call int) (int, string) {
		if request.Model == "claude-x" {
			return http.StatusNotFound, notFoundBody
		}
		return respondTokens(request, call)
	}
	stub := newCountServer(t, respondFallback)
	tests = append(tests, &testCase{
		_id:              "fallback-counts",
		_expectedApprox:  true,
		_expectedCalls:   2,
		_expectedCounter: contextrecord.CounterAnthropic,
		_expectedModels:  []string{"claude-x", fallbackModel},
		_expectedTokens:  5,
		_hasError:        false,

		counter: newTestCounter(t, stub),
		record:  claudeRecord("claude-x", "alpha"),
		stub:    stub,
	})

	// fallback-also-not-found
	respondNotFound := func(_ *countRequest, _ int) (int, string) {
		return http.StatusNotFound, notFoundBody
	}
	stub = newCountServer(t, respondNotFound)
	tests = append(tests, &testCase{
		_id:              "fallback-also-not-found",
		_expectedApprox:  false,
		_expectedCalls:   2,
		_expectedCounter: "",
		_expectedModels:  []string{"claude-x", fallbackModel},
		_expectedTokens:  0,
		_hasError:        true,

		counter: newTestCounter(t, stub),
		record:  claudeRecord("claude-x", "alpha"),
		stub:    stub,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.counter.Count(context.Background(), test.record)

			_, models, _ := test.stub.recorded()
			item := test.record.Agents[0].Items[0]
			assert.Equal(t, test._expectedApprox, item.Approx)
			assert.Equal(t, test._expectedCounter, item.Counter)
			assert.Equal(t, test._expectedTokens, item.Tokens)
			assert.Equal(t, test._hasError, item.Error != "")
			assert.Equal(t, test._expectedCalls, test.stub.callCount())
			assert.Equal(t, test._expectedModels, models)
			assert.Empty(t, test.counter.Notice())
		})
	}
}

func TestNew(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedMode   Mode
		_expectedNotice string

		mode Mode
	}

	tests := make([]*testCase, 0)

	// off
	tests = append(tests, &testCase{
		_id:             "off",
		_expectedMode:   ModeOff,
		_expectedNotice: noticeOff,

		mode: ModeOff,
	})

	// empty-defaults-off
	tests = append(tests, &testCase{
		_id:             "empty-defaults-off",
		_expectedMode:   ModeOff,
		_expectedNotice: noticeOff,

		mode: "",
	})

	// keychain — degrades to the unsupported notice off darwin and windows
	expectedKeychainMode := ModeKeychain
	expectedKeychainNotice := ""
	if !KeychainSupported {
		expectedKeychainMode = ModeOff
		expectedKeychainNotice = noticeUnsupported
	}
	tests = append(tests, &testCase{
		_id:             "keychain",
		_expectedMode:   expectedKeychainMode,
		_expectedNotice: expectedKeychainNotice,

		mode: ModeKeychain,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			counter, err := New(&Options{CacheDir: t.TempDir(), Mode: test.mode})

			require.NoError(t, err)
			assert.Equal(t, test._expectedMode, counter.mode)
			assert.Equal(t, test._expectedNotice, counter.Notice())
		})
	}

	// unknown-mode
	t.Run("unknown-mode", func(t *testing.T) {
		counter, err := New(&Options{CacheDir: t.TempDir(), Mode: "on"})

		assert.Nil(t, counter)
		assert.ErrorContains(t, err, `Unknown count-tokens mode "on"`)
	})
}

func TestNormalizeModel(t *testing.T) {
	type testCase struct {
		_id       string
		_expected modelRef

		model string
	}

	tests := make([]*testCase, 0)

	// suffix-stripped
	tests = append(tests, &testCase{
		_id:       "suffix-stripped",
		_expected: modelRef{approx: false, id: "claude-opus-5-5"},

		model: "claude-opus-5-5[1m]",
	})

	// plain-unchanged
	tests = append(tests, &testCase{
		_id:       "plain-unchanged",
		_expected: modelRef{approx: false, id: "claude-sonnet-5"},

		model: "claude-sonnet-5",
	})

	// empty-fallback-approx
	tests = append(tests, &testCase{
		_id:       "empty-fallback-approx",
		_expected: modelRef{approx: true, id: fallbackModel},

		model: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, normalizeModel(test.model))
		})
	}
}
