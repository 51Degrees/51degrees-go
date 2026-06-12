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

	"github.com/51Degrees/pipeline-go/core"
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

// SetMinimumEntriesPerMessage sets the batch size before a payload is sent.
func (b *Builder) SetMinimumEntriesPerMessage(n int) *Builder { b.minEntries = n; return b }

// SetMaximumQueueSize sets the size of the buffering queue.
func (b *Builder) SetMaximumQueueSize(n int) *Builder { b.maxQueueSize = n; return b }

// SetAddTimeout sets the timeout for adding to the queue.
func (b *Builder) SetAddTimeout(d time.Duration) *Builder { b.addTimeout = d; return b }

// SetTakeTimeout sets the timeout for taking items off the queue.
func (b *Builder) SetTakeTimeout(d time.Duration) *Builder { b.takeTimeout = d; return b }

// SetShareUsageURL sets the destination URL.
func (b *Builder) SetShareUsageURL(url string) *Builder { b.shareURL = url; return b }

// SetRepeatEvidenceInterval sets the sliding window during which identical data
// is suppressed.
func (b *Builder) SetRepeatEvidenceInterval(d time.Duration) *Builder {
	b.repeatInterval = d
	return b
}

// Build creates the Usage Sharing Element.
func (b *Builder) Build() *Element {
	cfg := config{
		includedQueryParams: toLowerSet(b.includedQueryParams),
		shareAllQueryParams: b.shareAllQueryParams,
		shareAllEvidence:    b.shareAllEvidence,
		blockedHeaders:      toLowerSet(b.blockedHeaders),
		sharePercentage:     b.sharePercentage,
		minEntries:          b.minEntries,
		maxQueueSize:        b.maxQueueSize,
		addTimeout:          b.addTimeout,
		takeTimeout:         b.takeTimeout,
		shareURL:            b.shareURL,
		repeatInterval:      b.repeatInterval,
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
	return b.Build(), nil
}

func toLowerSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[strings.ToLower(item)] = struct{}{}
	}
	return set
}
