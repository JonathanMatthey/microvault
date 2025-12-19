package store

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type memoryStore struct {
	mu    sync.RWMutex
	links map[string]Link
}

// NewMemory returns an in-memory share link store.
func NewMemory() Store {
	return &memoryStore{links: make(map[string]Link)}
}

func (m *memoryStore) Create(ownerID, uploadID string, ttl time.Duration) (Link, error) {
	// generate random 16-byte token
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Link{}, err
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	l := Link{
		Token:     token,
		OwnerID:   ownerID,
		UploadID:  uploadID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Downloads: 0,
	}
	m.mu.Lock()
	m.links[token] = l
	m.mu.Unlock()
	return l, nil
}

func (m *memoryStore) Get(token string) (Link, bool, error) {
	m.mu.RLock()
	l, ok := m.links[token]
	m.mu.RUnlock()
	if !ok {
		return Link{}, false, nil
	}
	if time.Now().After(l.ExpiresAt) {
		// expired: delete
		m.mu.Lock()
		delete(m.links, token)
		m.mu.Unlock()
		return Link{}, false, nil
	}
	return l, true, nil
}

func (m *memoryStore) IncrementDownloads(token string) error {
	m.mu.Lock()
	if l, ok := m.links[token]; ok {
		l.Downloads++
		m.links[token] = l
	}
	m.mu.Unlock()
	return nil
}
