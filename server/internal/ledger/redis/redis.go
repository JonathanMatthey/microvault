package redisledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/bosbaber/hackweek/selfstack/internal/ledger"
	redis "github.com/redis/go-redis/v9"
)

var (
	ErrNegativeAmount = errors.New("amount must be non-negative")
)

// Ledger implements a Redis-backed credit ledger.
// Keys are stored as <prefix><userID> with integer balances.
type Ledger struct {
	client    *redis.Client
	keyPrefix string
}

// New creates a new Redis-backed ledger.
func New(addr string, db int, password string, keyPrefix string) (*Ledger, error) {
	if addr == "" {
		return nil, fmt.Errorf("redis address is required")
	}
	if keyPrefix == "" {
		keyPrefix = "selfstack:credits:"
	}

	cli := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Ping to verify connection.
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Ledger{client: cli, keyPrefix: keyPrefix}, nil
}

func (l *Ledger) key(userID string) string {
	return l.keyPrefix + userID
}

func (l *Ledger) transactionKey(userID string) string {
	return l.keyPrefix + "tx:" + userID
}

// Balance returns the current balance, or 0 if no key exists.
func (l *Ledger) Balance(userID string) int64 {
	ctx := context.Background()
	val, err := l.client.Get(ctx, l.key(userID)).Result()
	if err == redis.Nil {
		return 0
	}
	if err != nil {
		// On Redis error, treat as 0 to avoid crashing callers.
		return 0
	}
	n, _ := strconv.ParseInt(val, 10, 64)
	return n
}

// Credit increments a user's balance.
func (l *Ledger) Credit(userID string, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	ctx := context.Background()
	return l.client.IncrBy(ctx, l.key(userID), amount).Err()
}

// Debit decrements a user's balance (can go negative).
func (l *Ledger) Debit(userID string, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	ctx := context.Background()
	return l.client.IncrBy(ctx, l.key(userID), -amount).Err()
}

// RecordTransaction appends a transaction to the user's history, keeping the latest 100.
func (l *Ledger) RecordTransaction(userID string, txn ledger.Transaction) error {
	ctx := context.Background()
	data, err := json.Marshal(txn)
	if err != nil {
		return err
	}
	key := l.transactionKey(userID)
	if err := l.client.LPush(ctx, key, data).Err(); err != nil {
		return err
	}
	return l.client.LTrim(ctx, key, 0, 99).Err()
}

// GetTransactionHistory returns up to limit recent transactions (newest first).
func (l *Ledger) GetTransactionHistory(userID string, limit int) ([]ledger.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	ctx := context.Background()
	key := l.transactionKey(userID)
	items, err := l.client.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	res := make([]ledger.Transaction, 0, len(items))
	for _, raw := range items {
		var txn ledger.Transaction
		if err := json.Unmarshal([]byte(raw), &txn); err == nil {
			res = append(res, txn)
		}
	}
	return res, nil
}

// ListAll returns all user balances as a map of userID -> balance.
func (l *Ledger) ListAll() (map[string]int64, error) {
	ctx := context.Background()
	result := make(map[string]int64)

	// Scan all keys matching the prefix
	var cursor uint64
	for {
		keys, nextCursor, err := l.client.Scan(ctx, cursor, l.keyPrefix+"*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan failed: %w", err)
		}

		// Get values for all keys in this batch
		for _, key := range keys {
			val, err := l.client.Get(ctx, key).Result()
			if err != nil {
				continue // Skip keys that disappeared
			}
			balance, _ := strconv.ParseInt(val, 10, 64)

			// Extract userID by removing prefix
			userID := key[len(l.keyPrefix):]
			result[userID] = balance
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return result, nil
}

var _ ledger.Ledger = (*Ledger)(nil)
