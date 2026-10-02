package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	BaseURL   string    `json:"baseUrl"`
	LocalURL  string    `json:"localUrl"`
	Port      int       `json:"port"`
	Tunnel    string    `json:"tunnel"`
	StartedAt time.Time `json:"startedAt"`
}

func statePath(home string) string {
	return filepath.Join(home, "state.json")
}

func Save(home string, value State) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(home), data, 0o600)
}

func Load(home string) (State, error) {
	var value State
	data, err := os.ReadFile(statePath(home))
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, err
	}
	if value.BaseURL == "" {
		return value, errors.New("invalid state file")
	}
	return value, nil
}

func Remove(home string) {
	_ = os.Remove(statePath(home))
}
