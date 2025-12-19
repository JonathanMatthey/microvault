package store

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

type redisStore struct {
	c      *redis.Client
	prefix string
}

// NewRedisStore creates a Store backed by Redis.
func NewRedisStore(addr string, db int, password, keyPrefix string) (Store, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db, Password: password})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	p := keyPrefix
	if p == "" {
		p = "microvault:wm:"
	}
	return &redisStore{c: rdb, prefix: p}, nil
}

func (s *redisStore) key(userID, incoming string) string { return s.prefix + userID + ":" + incoming }

func (s *redisStore) GetLastAmount(ctx context.Context, userID, incomingPayment string) (int64, error) {
	v, err := s.c.Get(ctx, s.key(userID, incomingPayment)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	// Redis stores strings; parse to int64
	var n int64
	_, err = fmt.Sscan(v, &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *redisStore) SetLastAmount(ctx context.Context, userID, incomingPayment string, amount int64) error {
	return s.c.Set(ctx, s.key(userID, incomingPayment), fmt.Sprintf("%d", amount), 0).Err()
}
