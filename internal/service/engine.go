package service

import (
	"context"
	"encoding/json"
	"errors"
	"hash"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Engine struct {
	nativeRelay            *nativeRelay
	importHashes           map[string][]hash.Hash
	mu                     sync.Mutex
	root                   string
	state                  State
	jobsByID               map[string]*Job // Derived index; guarded by mu, never persisted.
	active                 map[string]context.CancelFunc
	runner                 Runner
	online, wifi, charging bool
	stopped                bool
	wg                     sync.WaitGroup
	fault                  bool
}

func Open(root string, runner Runner) (*Engine, error) {
	if e := os.MkdirAll(filepath.Join(root, "media"), 0700); e != nil {
		return nil, e
	}
	if e := os.Chmod(root, 0700); e != nil {
		return nil, e
	}
	s := State{Version: 1, Options: defaults(), Jobs: []*Job{}}
	b, e := os.ReadFile(filepath.Join(root, "state.json"))
	if e == nil {
		// Defaults belong only to a new store, not a damaged persisted state.
		s = State{}
		if json.Unmarshal(b, &s) != nil || s.Version != 1 || !s.Options.valid() {
			return nil, errors.New("invalid state; restore backup")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if err := validateState(s); err != nil {
		return nil, err
	}
	en := &Engine{root: root, state: s, jobsByID: make(map[string]*Job, len(s.Jobs)), active: map[string]context.CancelFunc{}, importHashes: map[string][]hash.Hash{}, runner: runner}
	for _, j := range s.Jobs {
		en.jobsByID[j.ID] = j
		switch j.State {
		case "uploading", "preparing":
			j.State = "pending"
		case "committing":
			j.State = "failed"
			j.Error = "commit_outcome_unknown"
		case "importing":
			j.State = "cancelled"
			j.Error = "import_interrupted"
		}
		if j.CancelRequested && j.State == "pending" {
			j.State = "cancelled"
		}
		if j.State == "cancelled" || j.State == "completed" {
			_ = os.RemoveAll(en.jobDir(j.ID))
		}
	}
	if e = en.save(); e != nil {
		return nil, e
	}
	return en, nil
}
func (e *Engine) jobDir(id string) string { return filepath.Join(e.root, "media", id) }
func (e *Engine) find(id string) *Job     { return e.jobsByID[id] }
func (e *Engine) Close() {
	e.mu.Lock()
	e.stopped = true
	for _, c := range e.active {
		c()
	}
	e.mu.Unlock()
	e.wg.Wait()
}
func (e *Engine) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			e.Close()
			return
		case <-t.C:
			e.Tick()
		}
	}
}
