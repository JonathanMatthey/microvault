package incoming

import (
	"errors"
	"sync"
	"time"
)

// MemoryRepo is an in-memory Repository for tests and dev.
type MemoryRepo struct {
	mu    sync.RWMutex
	byURL map[string]PaymentRecord
}

func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{byURL: make(map[string]PaymentRecord)}
}

func (m *MemoryRepo) UpsertOnFetch(url, userID string, fetched FetchResult, now time.Time) (int64, PaymentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.byURL[url]
	if !ok {
		rec = PaymentRecord{
			URL:           url,
			UserID:        userID,
			AssetCode:     fetched.AssetCode,
			AssetScale:    fetched.AssetScale,
			LastValue:     fetched.TotalMinor,
			CreatedAt:     now,
			LastUpdatedAt: now,
			LastChangeAt:  time.Time{},
			Active:        true,
		}
		if fetched.TotalMinor > 0 {
			rec.LastChangeAt = now
		}
		m.byURL[url] = rec
		return fetched.TotalMinor, rec, nil
	}

	// Reactivate on event
	rec.Active = true
	// Ensure userID set if missing
	if rec.UserID == "" {
		rec.UserID = userID
	}
	// Asset code/scale: keep first seen unless empty; overwrite if changed but same URL
	if rec.AssetCode == "" {
		rec.AssetCode = fetched.AssetCode
	}
	if rec.AssetScale == 0 {
		rec.AssetScale = fetched.AssetScale
	}

	delta := fetched.TotalMinor - rec.LastValue
	if delta < 0 {
		// Non-monotonic; treat as no delta, but move pointer to fetched
		delta = 0
	}
	if fetched.TotalMinor > rec.LastValue {
		rec.LastChangeAt = now
		rec.LastValue = fetched.TotalMinor
	}
	rec.LastUpdatedAt = now
	m.byURL[url] = rec
	return delta, rec, nil
}

func (m *MemoryRepo) Get(url string) (PaymentRecord, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.byURL[url]
	return rec, ok, nil
}

func (m *MemoryRepo) ListActive() ([]PaymentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PaymentRecord, 0, len(m.byURL))
	for _, r := range m.byURL {
		if r.Active {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *MemoryRepo) MarkActive(url string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.byURL[url]
	if !ok {
		return errors.New("not found")
	}
	rec.Active = true
	rec.LastUpdatedAt = now
	m.byURL[url] = rec
	return nil
}

func (m *MemoryRepo) MarkInactive(url string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.byURL[url]
	if !ok {
		return errors.New("not found")
	}
	rec.Active = false
	rec.LastUpdatedAt = now
	m.byURL[url] = rec
	return nil
}
