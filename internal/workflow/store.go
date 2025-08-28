package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu         sync.RWMutex
	wsPath     string
	statePath  string
	eventsPath string
	lockPath   string
}

func NewStore(workspace string) *Store {
	root := filepath.Clean(workspace)
	loom := filepath.Join(root, ".loom")
	_ = os.MkdirAll(loom, 0o755)
	return &Store{
		wsPath:     root,
		statePath:  filepath.Join(loom, "workflow_state.v1.json"),
		eventsPath: filepath.Join(loom, "workflow_events.v1.ndjson"),
		lockPath:   filepath.Join(loom, "state.lock"),
	}
}

// WithLock executes fn while holding an exclusive flock on lockPath.
func (s *Store) WithLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := flockFile(f); err != nil {
		return errors.New("workflow state is locked by another process")
	}
	defer func() { _ = unlockFile(f) }()
	return fn()
}

func (s *Store) Load() (*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			now := time.Now().Unix()
			st := &WorkflowState{Version: 1, SessionID: time.Now().UTC().Format(time.RFC3339), CreatedAt: now, UpdatedAt: now, Phase: PhaseIdle}
			return st, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		now := time.Now().Unix()
		return &WorkflowState{Version: 1, CreatedAt: now, UpdatedAt: now, Phase: PhaseIdle}, nil
	}
	var st WorkflowState
	if err := json.Unmarshal(data, &st); err != nil {
		now := time.Now().Unix()
		return &WorkflowState{Version: 1, CreatedAt: now, UpdatedAt: now, Phase: PhaseAnalyze}, nil
	}
	return &st, nil
}

func (s *Store) Save(st *WorkflowState) error {
	return s.WithLock(func() error {
		tmp := s.statePath + ".tmp"
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, s.statePath)
	})
}

func (s *Store) AppendEvent(evt map[string]any) error {
	line, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return s.WithLock(func() error {
		f, err := os.OpenFile(s.eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
		return f.Sync()
	})
}

func (s *Store) ApplyEvent(ctx context.Context, evt map[string]any) error {
	_ = s.AppendEvent(evt)
	st, err := s.Load()
	if err != nil {
		return err
	}
	next := Reduce(evt, st)
	next.UpdatedAt = time.Now().Unix()
	if next.CreatedAt == 0 {
		next.CreatedAt = next.UpdatedAt
	}
	return s.Save(next)
}

func (s *Store) SnapshotFromEvents(n int) (*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Open(s.eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.Load()
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	start := 0
	if n > 0 && len(lines) > n {
		start = len(lines) - n
	}
	st, _ := s.Load()
	for _, ln := range lines[start:] {
		var e map[string]any
		if json.Unmarshal([]byte(ln), &e) == nil {
			st = Reduce(e, st)
		}
	}
	return st, nil
}
