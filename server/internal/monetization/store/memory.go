package store

import (
	"context"
	"sync"
)

type memStore struct {
	mu sync.RWMutex
	m  map[string]int64
}

// NewMemory returns an in-memory Store implementation
func NewMemory() Store {
	return &memStore{m: make(map[string]int64)}
}

func k(userID, incoming string) string { return userID + "|" + incoming }

func (s *memStore) GetLastAmount(ctx context.Context, userID, incomingPayment string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[k(userID, incomingPayment)], nil
}

func (s *memStore) SetLastAmount(ctx context.Context, userID, incomingPayment string, amount int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k(userID, incomingPayment)] = amount
	return nil
}
