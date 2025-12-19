package memory

import (
	"errors"
	"sync"

	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/ledger/fixed"
)

var (
	ErrNegativeAmount = errors.New("amount must be non-negative")
)

// Ledger is an in-memory, concurrency-safe Ledger implementation.
type Ledger struct {
	mu       sync.RWMutex
	balances map[string]int64
	history  map[string][]ledger.Transaction
}

// New creates a new in-memory ledger.
func New() *Ledger {
	return &Ledger{balances: make(map[string]int64), history: make(map[string][]ledger.Transaction)}
}

// Balance returns the current balance for a user (0 if not present).
func (l *Ledger) Balance(userID string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.balances[userID]
}

// Credit adds amount to a user's balance.
func (l *Ledger) Credit(userID string, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.balances[userID]
	next, err := fixed.Add(current, amount)
	if err != nil {
		return err
	}
	l.balances[userID] = next
	return nil
}

// Debit subtracts amount from a user's balance (can go negative).
func (l *Ledger) Debit(userID string, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.balances[userID]
	next, err := fixed.Sub(current, amount)
	if err != nil {
		return err
	}
	l.balances[userID] = next
	return nil
}

// ListAll returns all user balances.
func (l *Ledger) ListAll() (map[string]int64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	copy := make(map[string]int64, len(l.balances))
	for k, v := range l.balances {
		copy[k] = v
	}
	return copy, nil
}

// RecordTransaction stores a transaction in memory, trimming to the most recent 100 per user.
func (l *Ledger) RecordTransaction(userID string, txn ledger.Transaction) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	list := append([]ledger.Transaction{txn}, l.history[userID]...)
	if len(list) > 100 {
		list = list[:100]
	}
	l.history[userID] = list
	return nil
}

// GetTransactionHistory returns up to limit recent transactions.
func (l *Ledger) GetTransactionHistory(userID string, limit int) ([]ledger.Transaction, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	h := l.history[userID]
	if len(h) < limit {
		limit = len(h)
	}
	return append([]ledger.Transaction(nil), h[:limit]...), nil
}

var _ ledger.Ledger = (*Ledger)(nil)
