package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func (j *Job) resetRetry() {
	j.State = "pending"
	j.Attempts = 0
	j.Next = 0
	j.CancelRequested = false
}

func (e *Engine) Tick() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped || e.fault || e.state.Options.Paused || !e.online || (e.state.Options.WiFiOnly && !e.wifi) || (e.state.Options.ChargingOnly && !e.charging) {
		return
	}
	now := time.Now().Unix()
	for _, j := range e.state.Jobs {
		if len(e.active) >= e.state.Options.Concurrent {
			return
		}
		if j.State != "pending" || j.Next > now {
			continue
		}
		// Missing/expired host authorization waits without consuming retry budget.
		if e.nativeAuthorization(j.Account) == "waiting" {
			continue
		}
		j.State = "preparing"
		if j.Quality == "original" {
			j.OriginalPolicy = 1
		}
		j.Attempts++
		j.Uploaded = 0
		j.Error = ""
		if e.save() != nil {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		e.active[j.ID] = cancel
		paths := make([]string, 0, len(j.Resources))
		for _, f := range j.Resources {
			paths = append(paths, filepath.Join(e.jobDir(j.ID), f.Name))
		}
		e.wg.Add(1)
		go e.execute(ctx, *j, paths)
	}
}
func (e *Engine) execute(ctx context.Context, snapshot Job, paths []string) {
	defer e.wg.Done()
	key, err := e.runner(ctx, paths, snapshot.Account, snapshot.Quality, func(p Progress) {
		e.mu.Lock()
		defer e.mu.Unlock()
		j := e.find(snapshot.ID)
		if j == nil {
			return
		}
		old := j.State
		if p.State == "uploading" || p.State == "committing" || p.State == "preparing" {
			j.State = p.State
		}
		if len(p.MediaKeys) == 2 && p.MediaKeys[0] != "" && p.MediaKeys[1] != "" {
			j.MediaKeys = append([]string(nil), p.MediaKeys...)
		}
		if len(p.MediaKeys) == 0 {
			j.Uploaded = p.Uploaded
		}
		if p.Total > 0 {
			j.Total = p.Total
		}
		// Byte progress stays in memory; durable phase transitions protect crash recovery.
		if old != j.State && e.save() != nil {
			e.active[j.ID]()
		}
	})
	e.mu.Lock()
	defer e.mu.Unlock()
	j := e.find(snapshot.ID)
	delete(e.active, snapshot.ID)
	if j == nil {
		return
	}
	switch {
	case err == nil && key != "":
		e.state.CompletionRevision++
		j.State = "completed"
		j.MediaKey = key
		j.Uploaded = j.Total
		j.Error = ""
	case errors.Is(err, errRemoteComponentExists):
		j.State = "failed"
		j.Error = "remote_live_photo_component_exists"
	case j.State == "committing":
		j.State = "failed"
		j.Error = "commit_outcome_unknown"
	case j.CancelRequested:
		j.State = "cancelled"
		j.Error = ""
	case e.stopped || errors.Is(err, context.Canceled):
		// Lifecycle / network pauses do not consume the failure retry budget.
		if j.Attempts > 0 {
			j.Attempts--
		}
		j.State = "pending"
		j.Error = "paused"
	case e.nativeAuthorization(j.Account) == "waiting":
		if j.Attempts > 0 {
			j.Attempts--
		}
		j.State = "pending"
		j.Error = "waiting_for_native_auth"
		j.Next = 0
	case j.Attempts <= e.state.Options.Retries:
		j.State = "pending"
		j.Error = "upload_failed_retrying"
		j.Next = time.Now().Add(time.Second * time.Duration(1<<min(j.Attempts, 10))).Unix()
	default:
		j.State = "failed"
		j.Error = "upload_failed_check_account_and_network"
	}
	if e.save() == nil && (j.State == "completed" || j.State == "cancelled") {
		_ = os.RemoveAll(e.jobDir(j.ID))
	}
}
