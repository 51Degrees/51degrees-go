/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package usagesharing

import (
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Builder constructs a Usage Sharing Element.
type Builder struct {
	logger              *slog.Logger
	httpClient          *http.Client
	includedQueryParams []string
	shareAllQueryParams bool
	shareAllEvidence    bool
	blockedHeaders      []string
	sharePercentage     float64
	minEntries          int
	maxQueueSize        int
	addTimeout          time.Duration
	takeTimeout         time.Duration
	shareURL            string
	repeatInterval      time.Duration

	// maxQueueSizeSet is whether SetMaximumQueueSize was called, so the
	// default can follow the minimum entries per message otherwise.
	maxQueueSizeSet bool
	// recovery holds the back-off options once one is set, and is nil while
	// all are at their defaults.
	recovery *recoveryConfig
}

// NewBuilder creates a Usage Sharing Element builder with specification defaults.
func NewBuilder() *Builder {
	return &Builder{
		blockedHeaders:  []string{"cookies"},
		sharePercentage: DefaultSharePercentage,
		minEntries:      DefaultMinEntries,
		maxQueueSize:    DefaultMaxQueueSize,
		addTimeout:      DefaultAddTimeout,
		takeTimeout:     DefaultTakeTimeout,
		shareURL:        DefaultShareURL,
		repeatInterval:  DefaultRepeatInterval,
	}
}

// SetLogger sets the logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetHTTPClient overrides the HTTP client (mainly for testing).
func (b *Builder) SetHTTPClient(c *http.Client) *Builder { b.httpClient = c; return b }

// SetIncludedQueryStringParameters sets the allow-list of query/form parameters
// to share (in addition to those starting with 51D_).
func (b *Builder) SetIncludedQueryStringParameters(params ...string) *Builder {
	b.includedQueryParams = params
	return b
}

// SetShareAllQueryStringParameters shares all query/form parameters when true.
func (b *Builder) SetShareAllQueryStringParameters(v bool) *Builder {
	b.shareAllQueryParams = v
	return b
}

// SetShareAllEvidence shares all Evidence values when true.
func (b *Builder) SetShareAllEvidence(v bool) *Builder { b.shareAllEvidence = v; return b }

// SetBlockedHTTPHeaders sets HTTP headers that must not be shared.
func (b *Builder) SetBlockedHTTPHeaders(headers ...string) *Builder {
	b.blockedHeaders = headers
	return b
}

// SetSharePercentage sets the approximate proportion of requests to share.
func (b *Builder) SetSharePercentage(p float64) *Builder { b.sharePercentage = p; return b }

// SetMinimumEntriesPerMessage sets how many entries the queue must hold
// before a message is started. A message holds up to twice this number.
func (b *Builder) SetMinimumEntriesPerMessage(n int) *Builder { b.minEntries = n; return b }

// SetMaximumQueueSize sets the size of the buffering queue. The default is
// DefaultMaxQueueSize, or ten times the minimum entries per message if that
// is larger.
func (b *Builder) SetMaximumQueueSize(n int) *Builder {
	b.maxQueueSize = n
	b.maxQueueSizeSet = true
	return b
}

// SetAddTimeout sets the timeout for adding to the queue.
func (b *Builder) SetAddTimeout(d time.Duration) *Builder { b.addTimeout = d; return b }

// SetTakeTimeout sets how long to wait for each further entry while a
// message is being filled. Zero takes only the entries already queued.
func (b *Builder) SetTakeTimeout(d time.Duration) *Builder { b.takeTimeout = d; return b }

// SetShareUsageURL sets the destination URL.
func (b *Builder) SetShareUsageURL(url string) *Builder { b.shareURL = url; return b }

// SetRepeatEvidenceInterval sets how long after Evidence is shared that
// identical Evidence is not shared again. A repeat after that is shared and
// starts a new interval.
func (b *Builder) SetRepeatEvidenceInterval(d time.Duration) *Builder {
	b.repeatInterval = d
	return b
}

// recoveryOptions returns the back-off options to change, starting from the
// defaults.
func (b *Builder) recoveryOptions() *recoveryConfig {
	if b.recovery == nil {
		r := defaultRecovery()
		b.recovery = &r
	}
	return b.recovery
}

// SetFailuresToEnterRecovery sets how many failed sends within the failures
// window pause sending, from 1 to 100.
func (b *Builder) SetFailuresToEnterRecovery(n int) *Builder {
	b.recoveryOptions().failuresToEnterRecovery = n
	return b
}

// SetFailuresWindow sets the time over which failed sends are counted, 1
// second or more.
func (b *Builder) SetFailuresWindow(d time.Duration) *Builder {
	b.recoveryOptions().failuresWindow = d
	return b
}

// SetInitialRecoveryDelay sets the length of the first pause after failed
// sends, more than 0.
func (b *Builder) SetInitialRecoveryDelay(d time.Duration) *Builder {
	b.recoveryOptions().initialDelay = d
	return b
}

// SetMaximumRecoveryDelay sets the longest a pause can be, more than 0.
func (b *Builder) SetMaximumRecoveryDelay(d time.Duration) *Builder {
	b.recoveryOptions().maxDelay = d
	return b
}

// SetRecoveryMultiplier sets what a pause is multiplied by each time the
// trial send when it ends fails, 1 or more.
func (b *Builder) SetRecoveryMultiplier(m float64) *Builder {
	b.recoveryOptions().multiplier = m
	return b
}

// queueSize returns the queue size that was set, or the default.
func (b *Builder) queueSize() int {
	if b.maxQueueSizeSet {
		return b.maxQueueSize
	}
	return max(DefaultMaxQueueSize, queueSizePerMinimum*b.minEntries)
}

// Build creates the Usage Sharing Element. A back-off option outside its
// range is refused with a *core.ConfigurationError when the element is added
// to a Pipeline or processes.
func (b *Builder) Build() *Element {
	recovery := defaultRecovery()
	if b.recovery != nil {
		recovery = *b.recovery
	}
	cfg := config{
		includedQueryParams: toLowerSet(b.includedQueryParams),
		shareAllQueryParams: b.shareAllQueryParams,
		shareAllEvidence:    b.shareAllEvidence,
		blockedHeaders:      toLowerSet(b.blockedHeaders),
		sharePercentage:     b.sharePercentage,
		minEntries:          b.minEntries,
		maxQueueSize:        b.queueSize(),
		addTimeout:          b.addTimeout,
		takeTimeout:         b.takeTimeout,
		shareURL:            b.shareURL,
		repeatInterval:      b.repeatInterval,
		recovery:            recovery,
		err:                 recovery.validate(),
	}

	client := b.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	e := &Element{
		BaseElement: core.NewBaseElement(DataKey, nil, b.logger),
		cfg:         cfg,
		httpClient:  client,
		queue:       make(chan *record, cfg.maxQueueSize),
		stop:        make(chan struct{}),
		tracker:     newTracker(cfg.repeatInterval),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	e.SetEvidenceKeyFilter(core.NewFuncFilter(e.includeEvidence))
	return e
}

// includeEvidence implements the share-usage accepted-Evidence filter.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#accepted-evidence
func (e *Element) includeEvidence(key string) bool {
	if e.cfg.shareAllEvidence {
		return true
	}
	prefix, field := core.SplitEvidenceKey(key)
	field = strings.ToLower(field)
	switch prefix {
	case core.PrefixHeader:
		_, blocked := e.cfg.blockedHeaders[field]
		return !blocked
	case core.PrefixCookie:
		return strings.HasPrefix(field, "51d_")
	case core.PrefixQuery:
		if e.cfg.shareAllQueryParams {
			return true
		}
		if strings.HasPrefix(field, "51d_") {
			return true
		}
		_, included := e.cfg.includedQueryParams[field]
		return included
	default:
		return true
	}
}

// Factory builds a Usage Sharing Element from configuration.
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	b := NewBuilder().SetLogger(logger)
	if v, ok := core.StringListParam(params, "IncludedQueryStringParameters"); ok {
		b.SetIncludedQueryStringParameters(v...)
	}
	if v, ok := core.BoolParam(params, "ShareAllQueryStringParameters"); ok {
		b.SetShareAllQueryStringParameters(v)
	}
	if v, ok := core.BoolParam(params, "ShareAllEvidence"); ok {
		b.SetShareAllEvidence(v)
	}
	if v, ok := core.StringListParam(params, "BlockedHTTPHeaders"); ok {
		b.SetBlockedHTTPHeaders(v...)
	}
	if v, ok := core.FloatParam(params, "SharePercentage"); ok {
		b.SetSharePercentage(v)
	}
	if v, ok := core.IntParam(params, "MinimumEntriesPerMessage"); ok {
		b.SetMinimumEntriesPerMessage(v)
	}
	if v, ok := core.StringParam(params, "ShareUsageUrl"); ok {
		b.SetShareUsageURL(v)
	}
	if v, ok := core.IntParam(params, "RepeatEvidenceIntervalMinutes"); ok {
		b.SetRepeatEvidenceInterval(time.Duration(v) * time.Minute)
	}
	if v, ok := core.IntParam(params, "FailuresToEnterRecovery"); ok {
		b.SetFailuresToEnterRecovery(v)
	}
	if v, ok := core.FloatParam(params, "FailuresWindowSeconds"); ok {
		b.SetFailuresWindow(seconds(v))
	}
	if v, ok := core.FloatParam(params, "InitialRecoveryDelaySeconds"); ok {
		b.SetInitialRecoveryDelay(seconds(v))
	}
	if v, ok := core.FloatParam(params, "MaximumRecoveryDelaySeconds"); ok {
		b.SetMaximumRecoveryDelay(seconds(v))
	}
	if v, ok := core.FloatParam(params, "RecoveryMultiplier"); ok {
		b.SetRecoveryMultiplier(v)
	}
	e := b.Build()
	if e.cfg.err != nil {
		return nil, e.cfg.err
	}
	return e, nil
}

// seconds converts a number of seconds from configuration to a duration.
func seconds(v float64) time.Duration {
	return time.Duration(v * float64(time.Second))
}

func toLowerSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[strings.ToLower(item)] = struct{}{}
	}
	return set
}
