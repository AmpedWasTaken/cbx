package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AmpedWasTaken/cbx/internal/model"
)

var ErrNotFound = errors.New("callback not found")

type Store struct {
	root string
}

func New(home string) *Store {
	return &Store{root: filepath.Join(home, "data")}
}

func (s *Store) Init() error {
	if err := os.MkdirAll(filepath.Join(s.root, "callbacks"), 0o700); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(s.root, "events"), 0o700)
}

func (s *Store) CreateCallback(cb model.Callback) error {
	if cb.ID == "" {
		return errors.New("callback id is required")
	}
	path := filepath.Join(s.root, "callbacks", safeName(cb.ID)+".json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("callback %s already exists", cb.ID)
	}
	return writeJSONAtomic(path, cb)
}

func (s *Store) GetCallback(id string) (model.Callback, error) {
	var cb model.Callback
	path := filepath.Join(s.root, "callbacks", safeName(id)+".json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cb, ErrNotFound
	}
	if err != nil {
		return cb, err
	}
	if err := json.Unmarshal(data, &cb); err != nil {
		return cb, err
	}
	return cb, nil
}

func (s *Store) ListCallbacks() ([]model.Callback, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "callbacks"))
	if err != nil {
		return nil, err
	}
	out := make([]model.Callback, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, "callbacks", entry.Name()))
		if err != nil {
			return nil, err
		}
		var cb model.Callback
		if err := json.Unmarshal(data, &cb); err != nil {
			return nil, err
		}
		out = append(out, cb)
	}
	return out, nil
}

func (s *Store) DisableCallback(id string) error {
	cb, err := s.GetCallback(id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	cb.DisabledAt = &now
	return writeJSONAtomic(filepath.Join(s.root, "callbacks", safeName(cb.ID)+".json"), cb)
}

func (s *Store) AddEvent(event model.Event) error {
	if event.CallbackID == "" || event.ID == "" {
		return errors.New("event and callback ids are required")
	}
	dir := filepath.Join(s.root, "events", safeName(event.CallbackID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := fmt.Sprintf("%020d-%s.json", event.ReceivedAt.UnixNano(), safeName(event.ID))
	return writeJSONAtomic(filepath.Join(dir, name), event)
}

func (s *Store) ListEvents(callbackID string) ([]model.Event, error) {
	root := filepath.Join(s.root, "events")
	if callbackID != "" {
		return readEventDir(filepath.Join(root, safeName(callbackID)))
	}

	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []model.Event
	for _, entry := range dirs {
		if !entry.IsDir() {
			continue
		}
		events, err := readEventDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReceivedAt.Before(out[j].ReceivedAt) })
	return out, nil
}

func readEventDir(dir string) ([]model.Event, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []model.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]model.Event, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var event model.Event
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReceivedAt.Before(out[j].ReceivedAt) })
	return out, nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func safeName(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "..", "")
	return value
}
