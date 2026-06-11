package engines

import (
	"log/slog"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines/dataupdate"
)

// OnPremiseAspectEngine is an Aspect Engine that performs processing using a
// local data source, typically a data file that is periodically updated.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#on-premise-engines
type OnPremiseAspectEngine interface {
	AspectEngine
	dataupdate.Refreshable
	// DataFilePath returns the operational data file location, if any.
	DataFilePath() string
}

// BaseOnPremiseAspectEngine supplies the shared behavior for on-premise engines:
// a data file path and delegation of data refresh to functions supplied by the
// concrete engine. It implements dataupdate.Refreshable so it can be registered
// with a Data Update Service.
type BaseOnPremiseAspectEngine struct {
	*BaseAspectEngine
	dataFilePath     string
	refreshFromFile  func(path string) error
	refreshFromBytes func(data []byte) error
}

// NewBaseOnPremiseAspectEngine creates an on-premise engine base. The supplied
// refresh functions are invoked by the Data Update Service when new data is
// available; either may be nil if that mode is not supported.
func NewBaseOnPremiseAspectEngine(
	dataKey string,
	filter core.EvidenceKeyFilter,
	logger *slog.Logger,
	dataFilePath string,
	refreshFromFile func(path string) error,
	refreshFromBytes func(data []byte) error,
) *BaseOnPremiseAspectEngine {
	return &BaseOnPremiseAspectEngine{
		BaseAspectEngine: NewBaseAspectEngine(dataKey, filter, logger),
		dataFilePath:     dataFilePath,
		refreshFromFile:  refreshFromFile,
		refreshFromBytes: refreshFromBytes,
	}
}

// DataFilePath returns the operational data file location.
func (e *BaseOnPremiseAspectEngine) DataFilePath() string { return e.dataFilePath }

// RefreshFromFile reloads the engine's data from the given path.
func (e *BaseOnPremiseAspectEngine) RefreshFromFile(path string) error {
	if e.refreshFromFile == nil {
		return core.NewConfigurationError("engine '"+e.DataKey()+"' does not support file-based refresh", nil)
	}
	return e.refreshFromFile(path)
}

// RefreshFromBytes reloads the engine's data from an in-memory buffer.
func (e *BaseOnPremiseAspectEngine) RefreshFromBytes(data []byte) error {
	if e.refreshFromBytes == nil {
		return core.NewConfigurationError("engine '"+e.DataKey()+"' does not support memory-based refresh", nil)
	}
	return e.refreshFromBytes(data)
}
