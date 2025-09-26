package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

type State struct {
	Version        int               `json:"version"`
	Current        string            `json:"current"`
	Turn           int               `json:"turn"`
	CurrentTaskID  string            `json:"current_task_id"`  // WIP task ID (empty = no WIP)
	LeaseExpiresAt string            `json:"lease_expires_at"` // UTC RFC3339Nano (empty when no WIP)
	Inputs         map[string]string `json:"inputs"`
	LastArtifacts  map[string]string `json:"last_artifacts"`
	ArtifactsDir   string            `json:"artifacts_dir"`
	Meta           struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"meta"`
}

func loadState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	if st.ArtifactsDir == "" {
		st.ArtifactsDir = ".deespec/var/artifacts"
	}
	return &st, nil
}

func saveStateCAS(path string, st *State, prevVersion int) error {
	if st.Version != prevVersion {
		return fmt.Errorf("version changed (expected %d, got %d)", prevVersion, st.Version)
	}
	st.Version++
	st.Meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return fs.AtomicWriteJSON(path, st)
}

func nextStep(cur string, decision string) string {
	switch cur {
	case "plan":
		return "implement"
	case "implement":
		return "test"
	case "test":
		return "review"
	case "review":
		if decision == "OK" {
			return "done"
		}
		return "implement"
	case "done":
		return "done"
	default:
		return "plan"
	}
}
