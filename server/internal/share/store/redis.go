package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type redisStore struct {
	c      *redis.Client
	prefix string
}

// NewRedis creates a Redis-backed share link store. If prefix is empty, a default is used.
func NewRedis(addr string, db int, password, keyPrefix string) (Store, error) {
	cli := redis.NewClient(&redis.Options{Addr: addr, DB: db, Password: password})
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	p := keyPrefix
	if p == "" {
		p = "selfstack:share:"
	}
	return &redisStore{c: cli, prefix: p}, nil
}

func (r *redisStore) key(token string) string { return r.prefix + "token:" + token }

func (r *redisStore) Create(ownerID, uploadID string, ttl time.Duration) (Link, error) {
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
	bs, _ := json.Marshal(l)
	if err := r.c.Set(context.Background(), r.key(token), bs, ttl).Err(); err != nil {
		return Link{}, err
	}
	return l, nil
}

func (r *redisStore) Get(token string) (Link, bool, error) {
	bs, err := r.c.Get(context.Background(), r.key(token)).Bytes()
	if err == redis.Nil {
		return Link{}, false, nil
	}
	if err != nil {
		return Link{}, false, err
	}
	var l Link
	if err := json.Unmarshal(bs, &l); err != nil {
		return Link{}, false, err
	}
	// Also check TTL to be safe
	if time.Now().After(l.ExpiresAt) {
		_ = r.c.Del(context.Background(), r.key(token)).Err()
		return Link{}, false, nil
	}
	return l, true, nil
}

func (r *redisStore) IncrementDownloads(token string) error {
	key := r.key(token)
	ctx := context.Background()
	bs, err := r.c.Get(ctx, key).Bytes()
	if err != nil {
		return nil // ignore if missing
	}
	var l Link
	if err := json.Unmarshal(bs, &l); err != nil {
		return nil
	}
	l.Downloads++
	// Preserve remaining TTL
	ttl, _ := r.c.TTL(ctx, key).Result()
	out, _ := json.Marshal(l)
	return r.c.Set(ctx, key, out, ttl).Err()
}
