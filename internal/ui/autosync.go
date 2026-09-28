package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

const autoSyncDelay = 500 * time.Millisecond

type autoSyncState struct {
	mu         sync.Mutex
	timers     map[string]*time.Timer
	suppressed int
}

func (u *UI) scheduleAutoSync(feature string, apply func()) {
	if apply == nil || u.autoSync == nil || !u.autoSync.Checked || u.client == nil {
		return
	}
	u.autoSyncState.mu.Lock()
	defer u.autoSyncState.mu.Unlock()
	if u.autoSyncState.suppressed > 0 {
		return
	}
	if u.autoSyncState.timers == nil {
		u.autoSyncState.timers = make(map[string]*time.Timer)
	}
	if pending := u.autoSyncState.timers[feature]; pending != nil {
		pending.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(autoSyncDelay, func() {
		select {
		case <-u.done:
			return
		default:
		}
		fyne.Do(func() { u.dispatchAutoSync(feature, timer, apply) })
	})
	u.autoSyncState.timers[feature] = timer
}

func (u *UI) dispatchAutoSync(feature string, timer *time.Timer, apply func()) {
	u.autoSyncState.mu.Lock()
	if u.autoSyncState.timers[feature] != timer {
		u.autoSyncState.mu.Unlock()
		return
	}
	delete(u.autoSyncState.timers, feature)
	suppressed := u.autoSyncState.suppressed > 0
	u.autoSyncState.mu.Unlock()

	if suppressed || u.autoSync == nil || !u.autoSync.Checked || u.client == nil {
		return
	}
	if u.busy {
		u.scheduleAutoSync(feature, apply)
		return
	}
	apply()
}

func (u *UI) withoutAutoSync(fn func()) {
	u.autoSyncState.mu.Lock()
	u.autoSyncState.suppressed++
	u.autoSyncState.mu.Unlock()
	defer func() {
		u.autoSyncState.mu.Lock()
		u.autoSyncState.suppressed--
		u.autoSyncState.mu.Unlock()
	}()
	fn()
}

func (u *UI) cancelAutoSync() {
	u.autoSyncState.mu.Lock()
	defer u.autoSyncState.mu.Unlock()
	for feature, timer := range u.autoSyncState.timers {
		timer.Stop()
		delete(u.autoSyncState.timers, feature)
	}
}
