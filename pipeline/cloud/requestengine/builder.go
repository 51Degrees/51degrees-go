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
	"os"
	"strings"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// Builder constructs a CloudRequestEngine, which authenticates with a
// Resource Key, a License Key or both.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#credentials
type Builder struct {
	logger               *slog.Logger
	httpClient           *http.Client
	resourceKey          string
	licenseKey           string
	requestedProperties  []string
	endpoint             string
	dataEndpoint         string
	propertiesEndpoint   string
	evidenceKeysEndpoint string
	origin               string
	timeout              time.Duration

	failuresToEnterRecovery int
	failureWindow           time.Duration
	recoveryDuration        time.Duration
	recoveryMultiplier      float64
	maxRecoveryDuration     time.Duration
	now                     func() time.Time

	// state is a discovery state to use in place of the discovery requests.
	state *DiscoveryState
	// discovered is the discovery state of the last build, when known.
	discovered *DiscoveryState
}

// EnvCloudAPIURL is the environment variable that overrides the default cloud
// base endpoint when SetEndpoint is not used.
const EnvCloudAPIURL = "FOD_CLOUD_API_URL"

// NewBuilder creates a CloudRequestEngine builder with default endpoints and
// recovery settings. The base endpoint defaults to the FOD_CLOUD_API_URL
// environment variable when set, otherwise DefaultEndpoint; SetEndpoint still
// overrides either.
func NewBuilder() *Builder {
	endpoint := DefaultEndpoint
	if env := strings.TrimSpace(os.Getenv(EnvCloudAPIURL)); env != "" {
		endpoint = env
	}
	return &Builder{
		endpoint:                endpoint,
		timeout:                 DefaultTimeout,
		failuresToEnterRecovery: DefaultFailures,
		failureWindow:           DefaultFailureWindow,
		recoveryDuration:        DefaultRecovery,
		recoveryMultiplier:      DefaultRecoveryMultiplier,
		maxRecoveryDuration:     DefaultMaxRecovery,
	}
}

// SetLogger sets the engine's logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetHTTPClient overrides the HTTP client (mainly for testing).
func (b *Builder) SetHTTPClient(c *http.Client) *Builder { b.httpClient = c; return b }

// SetResourceKey sets the Resource Key used to authenticate requests and select
// the returned Properties.
func (b *Builder) SetResourceKey(key string) *Builder { b.resourceKey = key; return b }

// SetLicenseKey sets the License Key used to authenticate requests. With no
// Resource Key, SetRequestedProperties must name the Properties wanted,
// because a License Key names none of its own. Beside a Resource Key, the
// answer holds the Resource Key's Properties plus the Requested Properties
// the License Key grants.
func (b *Builder) SetLicenseKey(key string) *Builder { b.licenseKey = key; return b }

// SetRequestedProperties sets the Properties to ask the cloud service for, as
// fully qualified names such as "device.ismobile". They are required with a
// License Key alone, added to the Resource Key's Properties when both keys
// are set, and refused with a Resource Key and no License Key, which the
// cloud service answers with everything the key carries whatever the list
// says.
func (b *Builder) SetRequestedProperties(names ...string) *Builder {
	b.requestedProperties = append([]string(nil), names...)
	return b
}

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

// SetRecoveryMultiplier sets the factor by which the recovery duration grows
// after each consecutive trip, resetting on the next success. A multiplier of 1
// (the default) keeps the recovery duration fixed; a larger value gives
// exponential backoff.
func (b *Builder) SetRecoveryMultiplier(m float64) *Builder { b.recoveryMultiplier = m; return b }

// SetMaxRecoveryDuration caps the recovery duration once it has grown through
// the multiplier. Zero or negative leaves it uncapped.
func (b *Builder) SetMaxRecoveryDuration(d time.Duration) *Builder {
	b.maxRecoveryDuration = d
	return b
}

// setClock overrides the clock used for recovery timing (testing only).
func (b *Builder) setClock(now func() time.Time) *Builder { b.now = now; return b }

// SetDiscoveryState gives Build the accepted Evidence keys and accessible
// Properties that an earlier build discovered, read from DiscoveryState, so
// Build makes no discovery request. They depend only on the credentials, so
// use a state from a build with the same keys. Nil, the default, makes Build
// fetch them from the cloud service. Build refuses a state whose
// EvidenceKeys or Products is nil, such as one read from an empty JSON
// object, because the engine would never fetch what is missing. An empty
// list or map is kept, as it is when the cloud service gives one.
func (b *Builder) SetDiscoveryState(s *DiscoveryState) *Builder { b.state = s; return b }

// DiscoveryState returns what the last Build discovered, or was given with
// SetDiscoveryState, ready to keep for a later build. It is nil before a
// build, and after a build that could not fetch both because of a transient
// failure.
func (b *Builder) DiscoveryState() *DiscoveryState { return b.discovered }

// Build creates the CloudRequestEngine and fetches its accessible Properties
// and accepted Evidence keys from the cloud service. A combination of Resource
// Key, License Key and Requested Properties that the specification's table
// marks invalid fails the build with a *core.ConfigurationError, before any
// request. A key or name that is empty or only whitespace counts as absent.
// A 4xx answer to either request,
// such as for an invalid Resource Key, fails the build with a
// *core.ConfigurationError wrapping the *CloudRequestError. Any other failure
// is logged as a warning and the build completes, and the engine fetches what
// is missing on first use.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#revised-design-eager-discovery-at-build-time-transient-failure-fallback
func (b *Builder) Build() (*CloudRequestEngine, error) {
	b.discovered = nil
	resourceKey := strings.TrimSpace(b.resourceKey)
	licenseKey := strings.TrimSpace(b.licenseKey)
	requested := trimmedNames(b.requestedProperties)
	if err := checkCredentials(resourceKey, licenseKey, requested); err != nil {
		return nil, err
	}
	if b.state != nil && (b.state.EvidenceKeys == nil || b.state.Products == nil) {
		return nil, core.NewConfigurationError("the discovery state given to "+
			"the builder lacks its Evidence keys or its Products, so build "+
			"without it to fetch them from the cloud service", nil)
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

	e := &CloudRequestEngine{
		httpClient:              client,
		resourceKey:             resourceKey,
		licenseKey:              licenseKey,
		requestedProperties:     requested,
		origin:                  b.origin,
		dataEndpoint:            dataEndpoint,
		propertiesEndpoint:      propertiesEndpoint,
		evidenceKeysEndpoint:    evidenceKeysEndpoint,
		failuresToEnterRecovery: b.failuresToEnterRecovery,
		failureWindow:           b.failureWindow,
		recoveryDuration:        b.recoveryDuration,
		recoveryMultiplier:      b.recoveryMultiplier,
		maxRecoveryDuration:     b.maxRecoveryDuration,
		now:                     now,
	}
	// The engine accepts the Evidence keys the cloud service gives, which
	// the filter reads from the engine as discovery completes.
	e.BaseAspectEngine = engines.NewBaseAspectEngine(DataKey,
		acceptedEvidence{e}, b.logger)
	e.SetDataSourceTier("cloud")

	if b.state != nil {
		e.useState(b.state)
	} else if err := e.discoverAtBuild(); err != nil {
		return nil, err
	}
	b.discovered = e.discoveryState()
	return e, nil
}

// The messages for the combinations of credentials Build refuses. Each says
// why the combination cannot work, because the rule on its own reads as
// arbitrary.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#accepted-combinations
const (
	messageNoCredential = "CloudRequestEngine needs a Resource Key or a " +
		"License Key to authenticate with, as the cloud service answers 401 " +
		"to a request that carries neither. Set one with SetResourceKey or " +
		"SetLicenseKey. Create a Resource Key for free at " +
		"https://configure.51degrees.com/?utm_source=code&utm_medium=package&utm_campaign=51degrees-go&utm_content=pipeline-cloud-requestengine-builder.go&utm_term=no-credential"
	messageLicenseKeyNeedsProperties = "CloudRequestEngine has a License " +
		"Key and no Requested Properties. A License Key names no Properties " +
		"of its own, so the cloud service answers 400 to every request made " +
		"with one that does not list the Properties it wants. Call " +
		"SetRequestedProperties with the fully qualified names wanted, for " +
		"example \"device.ismobile\" and \"device.iscrawler\"."
	messagePropertiesIgnoredWithResourceKey = "CloudRequestEngine has " +
		"Requested Properties and a Resource Key but no License Key. The " +
		"cloud service ignores the list unless a License Key is sent, and " +
		"answers with everything the Resource Key carries, so the caller " +
		"would believe the response had changed when it had not. Either " +
		"leave out SetRequestedProperties, or call SetLicenseKey with a " +
		"License Key that grants the Properties."
)

// checkCredentials refuses the combinations the specification's table marks
// invalid, so a deployment sees one configuration error when the engine is
// built rather than a failure on every request it serves.
func checkCredentials(resourceKey, licenseKey string, requested []string) error {
	switch {
	case resourceKey == "" && licenseKey == "":
		return core.NewConfigurationError(messageNoCredential, nil)
	case resourceKey == "" && len(requested) == 0:
		return core.NewConfigurationError(messageLicenseKeyNeedsProperties, nil)
	case resourceKey != "" && licenseKey == "" && len(requested) > 0:
		return core.NewConfigurationError(messagePropertiesIgnoredWithResourceKey, nil)
	}
	return nil
}

// trimmedNames returns names with each trimmed and the empty ones left out.
func trimmedNames(names []string) []string {
	var out []string
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
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
	if v, ok := core.StringParam(params, "LicenseKey"); ok {
		b.SetLicenseKey(v)
	}
	if v, ok := core.StringListParam(params, "RequestedProperties"); ok {
		b.SetRequestedProperties(v...)
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
	// Return a nil element on failure, not a nil *CloudRequestEngine, which
	// would make a non-nil core.FlowElement.
	e, err := b.Build()
	if err != nil {
		return nil, err
	}
	return e, nil
}
