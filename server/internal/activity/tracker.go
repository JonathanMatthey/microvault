package activity

import "sync"

// Tracker records last active timestamps per user.
type Tracker struct {
	mu   sync.RWMutex
	last map[string]int64
}

// New returns a new Tracker.
func New() *Tracker {
	return &Tracker{last: make(map[string]int64)}
}

// Record sets the last active timestamp for a user.
func (t *Tracker) Record(userID string, ts int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last[userID] = ts
}

// LastActive returns the last active timestamp (unix seconds). Zero if none.
func (t *Tracker) LastActive(userID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.last[userID]
}
