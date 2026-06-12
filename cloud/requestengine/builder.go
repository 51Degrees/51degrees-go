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

package requestengine

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
)

// Builder constructs a CloudRequestEngine. The Resource Key is required.
type Builder struct {
	logger               *slog.Logger
	httpClient           *http.Client
	resourceKey          string
	endpoint             string
	dataEndpoint         string
	propertiesEndpoint   string
	evidenceKeysEndpoint string
	origin               string
	timeout              time.Duration

	failuresToEnterRecovery int
	failureWindow           time.Duration
	recoveryDuration        time.Duration
	now                     func() time.Time
}

// NewBuilder creates a CloudRequestEngine builder with default endpoints and
// recovery settings.
func NewBuilder() *Builder {
	return &Builder{
		endpoint:                DefaultEndpoint,
		timeout:                 DefaultTimeout,
		failuresToEnterRecovery: DefaultFailures,
		failureWindow:           DefaultFailureWindow,
		recoveryDuration:        DefaultRecovery,
	}
}

// SetLogger sets the engine's logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetHTTPClient overrides the HTTP client (mainly for testing).
func (b *Builder) SetHTTPClient(c *http.Client) *Builder { b.httpClient = c; return b }

// SetResourceKey sets the Resource Key used to authenticate requests and select
// the returned Properties.
func (b *Builder) SetResourceKey(key string) *Builder { b.resourceKey = key; return b }

// SetEndpoint sets the base URL; endpoint paths are derived from it unless set
// explicitly.
func (b *Builder) SetEndpoint(url string) *Builder { b.endpoint = url; return b }

// SetDataEndpoint overrides the data (json) endpoint URL.
func (b *Builder) SetDataEndpoint(url string) *Builder { b.dataEndpoint = url; return b }

// SetPropertiesEndpoint overrides the accessible properties endpoint URL.
func (b *Builder) SetPropertiesEndpoint(url string) *Builder { b.propertiesEndpoint = url; return b }

// SetEvidenceKeysEndpoint overrides the evidence keys endpoint URL.
func (b *Builder) SetEvidenceKeysEndpoint(url string) *Builder {
	b.evidenceKeysEndpoint = url
	return b
}

// SetCloudRequestOrigin sets the value of the Origin header.
func (b *Builder) SetCloudRequestOrigin(origin string) *Builder { b.origin = origin; return b }

// SetTimeout sets the HTTP request timeout.
func (b *Builder) SetTimeout(d time.Duration) *Builder { b.timeout = d; return b }

// SetFailuresToEnterRecovery sets the number of failures within the window that
// trigger recovery mode.
func (b *Builder) SetFailuresToEnterRecovery(n int) *Builder { b.failuresToEnterRecovery = n; return b }

// SetFailureWindow sets the window over which failures are counted.
func (b *Builder) SetFailureWindow(d time.Duration) *Builder { b.failureWindow = d; return b }

// SetRecoveryDuration sets how long recovery mode lasts. Zero or negative
// disables recovery mode.
func (b *Builder) SetRecoveryDuration(d time.Duration) *Builder { b.recoveryDuration = d; return b }

// setClock overrides the clock used for recovery timing (testing only).
func (b *Builder) setClock(now func() time.Time) *Builder { b.now = now; return b }

// Build creates the CloudRequestEngine.
func (b *Builder) Build() (*CloudRequestEngine, error) {
	if b.resourceKey == "" {
		return nil, core.NewConfigurationError("a Resource Key is required to build a CloudRequestEngine", nil)
	}
	base := strings.TrimRight(b.endpoint, "/") + "/"
	dataEndpoint := orDefault(b.dataEndpoint, base+"json")
	propertiesEndpoint := orDefault(b.propertiesEndpoint, base+"accessibleproperties")
	evidenceKeysEndpoint := orDefault(b.evidenceKeysEndpoint, base+"evidencekeys")

	client := b.httpClient
	if client == nil {
		client = &http.Client{Timeout: b.timeout}
	}
	now := b.now
	if now == nil {
		now = time.Now
	}

	// The Cloud Request Engine forwards all evidence whose keys the server might
	// accept; until the evidence-keys metadata loads it accepts everything, so a
	// permissive filter is used at the element level.
	filter := core.NewFuncFilter(func(string) bool { return true })

	e := &CloudRequestEngine{
		BaseAspectEngine:        engines.NewBaseAspectEngine(DataKey, filter, b.logger),
		httpClient:              client,
		resourceKey:             b.resourceKey,
		origin:                  b.origin,
		dataEndpoint:            dataEndpoint,
		propertiesEndpoint:      propertiesEndpoint,
		evidenceKeysEndpoint:    evidenceKeysEndpoint,
		failuresToEnterRecovery: b.failuresToEnterRecovery,
		failureWindow:           b.failureWindow,
		recoveryDuration:        b.recoveryDuration,
		now:                     now,
	}
	e.SetDataSourceTier("cloud")
	return e, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Factory builds a CloudRequestEngine from configuration parameters for use with
// core.BuilderRegistry.
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	b := NewBuilder().SetLogger(logger)
	if v, ok := core.StringParam(params, "ResourceKey"); ok {
		b.SetResourceKey(v)
	}
	if v, ok := core.StringParam(params, "EndPoint"); ok {
		b.SetEndpoint(v)
	}
	if v, ok := core.StringParam(params, "CloudRequestOrigin"); ok {
		b.SetCloudRequestOrigin(v)
	}
	if v, ok := core.IntParam(params, "TimeoutSeconds"); ok {
		b.SetTimeout(time.Duration(v) * time.Second)
	}
	if v, ok := core.IntParam(params, "FailuresToEnterRecovery"); ok {
		b.SetFailuresToEnterRecovery(v)
	}
	if v, ok := core.IntParam(params, "FailuresWindowSeconds"); ok {
		b.SetFailureWindow(time.Duration(v) * time.Second)
	}
	if v, ok := core.FloatParam(params, "RecoverySeconds"); ok {
		b.SetRecoveryDuration(time.Duration(v * float64(time.Second)))
	}
	return b.Build()
}
