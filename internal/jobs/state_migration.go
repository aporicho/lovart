package jobs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

const currentRunStateVersion = 1

// RunStateSchemaError describes an incompatible persisted run state schema.
type RunStateSchemaError struct {
	Version int
	Message string
}

func (e *RunStateSchemaError) Error() string {
	if e.Version == 0 {
		return fmt.Sprintf("jobs: state schema error: %s", e.Message)
	}
	return fmt.Sprintf("jobs: state schema version %d: %s", e.Version, e.Message)
}

func decodeRunState(data []byte, runDir string) (*RunState, error) {
	var state RunState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("jobs: parse state: %w", err)
	}
	if err := migrateRunState(&state); err != nil {
		return nil, err
	}
	setRunStateDefaults(&state, runDir)
	if err := validateRunStateSchema(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func migrateRunState(state *RunState) error {
	for {
		switch state.Version {
		case 0:
			migrateRunStateUnversionedToV1(state)
		case currentRunStateVersion:
			return nil
		default:
			if state.Version > currentRunStateVersion {
				return &RunStateSchemaError{
					Version: state.Version,
					Message: fmt.Sprintf("unsupported future version; current version is %d", currentRunStateVersion),
				}
			}
			return &RunStateSchemaError{Version: state.Version, Message: "unsupported version"}
		}
	}
}

func migrateRunStateUnversionedToV1(state *RunState) {
	state.Version = currentRunStateVersion
}

func setRunStateDefaults(state *RunState, runDir string) {
	if state.Version == 0 {
		state.Version = currentRunStateVersion
	}
	if state.RunDir == "" {
		state.RunDir = runDir
	}
	if state.StateFile == "" {
		state.StateFile = filepath.Join(runDir, stateFileName)
	}
}

func validateRunStateSchema(state *RunState) error {
	if state.Version != currentRunStateVersion {
		return &RunStateSchemaError{
			Version: state.Version,
			Message: fmt.Sprintf("expected current version %d after migration", currentRunStateVersion),
		}
	}
	return nil
}

func prepareRunStateForSave(state *RunState) {
	if state.Version == 0 {
		state.Version = currentRunStateVersion
	}
}
