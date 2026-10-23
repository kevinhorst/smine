// Package tokencount counts injected-context tokens with each provider's own mechanism.
package tokencount

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tiktoken-go/tokenizer"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

// Mode is the routines.count_tokens setting value.
type Mode string

const (
	ModeKeychain Mode = "keychain"
	ModeOff      Mode = "off"

	// SkillCountModel stands for every current Claude model: they share the tokenizer.
	SkillCountModel = fallbackModel

	noticeOff         = "Claude token counting is off — set routines.count_tokens to keychain"
	noticeUnsupported = "Claude token counting: macOS keychain only"
	poolWorkers       = 4
	suffix1m          = "[1m]"
)

// Options wires a Counter.
type Options struct {
	CacheDir string
	Mode     Mode
}

// Counter counts a record's items; safe for concurrent page requests.
type Counter struct {
	anthropic *anthropicClient
	cache     *cache
	codec     tokenizer.Codec
	guard     *guard
	mode      Mode
	mu        sync.Mutex
	notice    string
	readKey   func(context.Context) (string, error)
}

// New degrades an unsupported keychain platform to a notice.
func New(opts *Options) (*Counter, error) {
	mode := opts.Mode
	if mode == "" {
		mode = ModeOff
	}
	if mode != ModeOff && mode != ModeKeychain {
		return nil, fmt.Errorf("New: Unknown count-tokens mode %q (off | keychain)", mode)
	}
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	counter := &Counter{
		cache:   loadCache(filepath.Join(opts.CacheDir, cacheFile)),
		codec:   codec,
		guard:   newGuard(time.Now),
		mode:    mode,
		readKey: readKeychainKey,
	}
	switch {
	case mode == ModeOff:
		counter.notice = noticeOff
	case !KeychainSupported:
		counter.notice = noticeUnsupported
		counter.mode = ModeOff
	}
	return counter, nil
}

func (c *Counter) Count(ctx context.Context, record *contextrecord.Record) {
	switch record.Agent {
	case contextrecord.SessionClaude:
		c.countAnthropic(ctx, record)
	case contextrecord.SessionCodex:
		c.countO200k(record)
	default:
		markUncounted(record, contextrecord.UncountedDisabled)
	}
	record.Recompute()
}

// CountCached never calls the API, so a page request never waits on it.
func (c *Counter) CountCached(ctx context.Context, record *contextrecord.Record) int {
	queued := 0
	switch record.Agent {
	case contextrecord.SessionClaude:
		queued = c.fillCached(ctx, record)
	case contextrecord.SessionCodex:
		c.countO200k(record)
	default:
		markUncounted(record, contextrecord.UncountedDisabled)
	}
	record.Recompute()
	return queued
}

// CountText returns an uncounted result as an error, never a silent zero.
func (c *Counter) CountText(ctx context.Context, model, text string) (int, error) {
	if !c.ensureClient(ctx) {
		return 0, errors.New("Counter.CountText: " + c.Notice())
	}

	key := cacheKey(contextrecord.CounterAnthropic, model, text)
	if tokens, isCached := c.cache.get(key); isCached {
		return tokens, nil
	}

	item := contextrecord.NewItem(contextrecord.KindSkill, "", "", text)
	unit := &countWork{item: &item, key: key, model: modelRef{id: model}}
	result := c.countOne(ctx, unit)
	switch {
	case result.uncounted != "":
		return 0, fmt.Errorf("Counter.CountText: Not counted (%s)", result.uncounted)
	case result.err != nil:
		return 0, fmt.Errorf("Counter.CountText: %w", result.err)
	}

	c.cache.put(key, result.tokens)
	if err := c.cache.save(); err != nil {
		log.Printf("Counter.CountText: Token cache not saved: %v", err)
	}
	return result.tokens, nil
}

// Notice is the one-line reason Claude counting is not running, or "".
func (c *Counter) Notice() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.notice
}

// countAnthropic counts a Claude record: cache first, caps, then the pool.
func (c *Counter) countAnthropic(ctx context.Context, record *contextrecord.Record) {
	if !c.ensureClient(ctx) {
		markUncounted(record, contextrecord.UncountedDisabled)
		return
	}

	var work []*countWork
	byKey := map[string]*countWork{}
	for agentIndex := range record.Agents {
		agent := &record.Agents[agentIndex]
		model := normalizeModel(agent.Model)
		for itemIndex := range agent.Items {
			item := &agent.Items[itemIndex]
			if item.Counter != "" {
				continue
			}
			if item.Chars > maxItemRunes {
				item.Uncounted = contextrecord.UncountedSize
				continue
			}
			key := cacheKey(contextrecord.CounterAnthropic, model.id, item.Text())
			if tokens, isCached := c.cache.get(key); isCached {
				item.SetTokens(contextrecord.CounterAnthropic, tokens, model.approx)
				continue
			}
			if unit, isQueued := byKey[key]; isQueued {
				unit.twins = append(unit.twins, item)
				continue
			}
			if len(work) >= MaxItemsPerReport {
				item.Uncounted = contextrecord.UncountedCap
				continue
			}
			unit := &countWork{item: item, key: key, model: model}
			byKey[key] = unit
			work = append(work, unit)
		}
	}

	for _, result := range c.runPool(ctx, work) {
		c.apply(result)
	}
	if err := c.cache.save(); err != nil {
		log.Printf("Counter.countAnthropic: Token cache not saved: %v", err)
	}
}

// countO200k is exact only for the model families tiktoken maps.
func (c *Counter) countO200k(record *contextrecord.Record) {
	for agentIndex := range record.Agents {
		agent := &record.Agents[agentIndex]
		isApprox := encodingApprox(agent.Model)
		for itemIndex := range agent.Items {
			item := &agent.Items[itemIndex]
			if item.Counter != "" {
				continue
			}
			tokens, err := c.codec.Count(item.Text())
			if err != nil {
				item.Error = fmt.Sprintf("o200k_base: %v", err)
				continue
			}
			item.SetTokens(contextrecord.CounterO200k, tokens, isApprox)
		}
	}
}

func (c *Counter) countOne(ctx context.Context, unit *countWork) countResult {
	result := countResult{work: unit}
	if c.guard.isTripped() {
		result.uncounted = contextrecord.UncountedDisabled
		return result
	}
	if !c.guard.take() {
		result.uncounted = contextrecord.UncountedRate
		return result
	}

	tokens, err := c.anthropic.countTokens(ctx, unit.model.id, unit.item.Text())
	var apiErr *apiError
	isUnknownModel := errors.As(err, &apiErr) && apiErr.Type == errorTypeNotFound && !unit.model.approx
	if isUnknownModel {
		tokens, err = c.anthropic.countTokens(ctx, fallbackModel, unit.item.Text())
		result.approx = true
	}
	if err != nil {
		result.err = err
		if c.guard.recordFailure(err) {
			c.setNotice(fmt.Sprintf("Claude token counting paused after %d consecutive API failures (%v) — restart the config server to retry", breakerTrips, err))
		}
		return result
	}
	c.guard.recordSuccess()
	result.tokens = tokens
	return result
}

// ensureClient disables this report on a failed read; the next report retries.
func (c *Counter) ensureClient(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.anthropic != nil {
		return true
	}
	if c.mode == ModeOff {
		return false
	}

	key, err := c.readKey(ctx)
	if err != nil {
		c.notice = fmt.Sprintf("Claude token counting disabled: %v", err)
		return false
	}
	c.notice = ""
	c.anthropic = newAnthropicClient(key, countTokensURL)
	return true
}

func (c *Counter) fillCached(ctx context.Context, record *contextrecord.Record) int {
	if !c.ensureClient(ctx) || c.guard.isTripped() {
		markUncounted(record, contextrecord.UncountedDisabled)
		return 0
	}

	queued := 0
	for agentIndex := range record.Agents {
		agent := &record.Agents[agentIndex]
		model := normalizeModel(agent.Model)
		for itemIndex := range agent.Items {
			item := &agent.Items[itemIndex]
			if item.Counter != "" {
				continue
			}
			if item.Chars > maxItemRunes {
				item.Uncounted = contextrecord.UncountedSize
				continue
			}
			key := cacheKey(contextrecord.CounterAnthropic, model.id, item.Text())
			if tokens, isCached := c.cache.get(key); isCached {
				item.SetTokens(contextrecord.CounterAnthropic, tokens, model.approx)
				continue
			}
			item.Uncounted = contextrecord.UncountedQueued
			queued++
		}
	}
	return queued
}

// runPool counts work units on a fixed pool of workers over a work channel.
func (c *Counter) runPool(ctx context.Context, work []*countWork) []countResult {
	workChan := make(chan *countWork, len(work))
	for _, unit := range work {
		workChan <- unit
	}
	close(workChan)

	resultChan := make(chan countResult, len(work))
	var wg sync.WaitGroup
	for range poolWorkers {
		wg.Go(func() {
			for unit := range workChan {
				resultChan <- c.countOne(ctx, unit)
			}
		})
	}
	wg.Wait()
	close(resultChan)

	results := make([]countResult, 0, len(work))
	for result := range resultChan {
		results = append(results, result)
	}
	return results
}

func (c *Counter) apply(result countResult) {
	items := append([]*contextrecord.Item{result.work.item}, result.work.twins...)
	for _, item := range items {
		switch {
		case result.uncounted != "":
			item.Uncounted = result.uncounted
		case result.err != nil:
			item.Error = result.err.Error()
		default:
			item.SetTokens(contextrecord.CounterAnthropic, result.tokens, result.approx || result.work.model.approx)
		}
	}

	isCounted := result.uncounted == "" && result.err == nil
	if isCounted {
		c.cache.put(result.work.key, result.tokens)
	}
}

func (c *Counter) setNotice(notice string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.notice == "" {
		c.notice = notice
	}
}

// countResult is one pool result; exactly one of tokens, err, uncounted is set.
type countResult struct {
	approx    bool
	err       error
	tokens    int
	uncounted string
	work      *countWork
}

// countWork's twins share text and model and take the unit's result instead of their own call.
type countWork struct {
	item  *contextrecord.Item
	key   string
	model modelRef
	twins []*contextrecord.Item
}

// modelRef is a count_tokens model id; approx marks the fallback.
type modelRef struct {
	approx bool
	id     string
}

// markUncounted stamps every uncounted item with reason.
func markUncounted(record *contextrecord.Record, reason string) {
	for agentIndex := range record.Agents {
		for itemIndex := range record.Agents[agentIndex].Items {
			item := &record.Agents[agentIndex].Items[itemIndex]
			if item.Counter == "" {
				item.Uncounted = reason
			}
		}
	}
}

// normalizeModel strips the harness's [1m] suffix.
func normalizeModel(model string) modelRef {
	id := strings.TrimSuffix(model, suffix1m)
	if id == "" {
		return modelRef{approx: true, id: fallbackModel}
	}
	return modelRef{id: id}
}
