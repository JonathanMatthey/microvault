package incoming

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	c      *redis.Client
	prefix string
}

// NewRedisRepo creates a repository backed by Redis.
func NewRedisRepo(addr string, db int, password, keyPrefix string) (*RedisRepo, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db, Password: password})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	p := keyPrefix
	if p == "" {
		p = "selfstack:ip:"
	}
	return &RedisRepo{c: rdb, prefix: p}, nil
}

func (r *RedisRepo) keyFor(url string) string {
	h := sha1.Sum([]byte(url))
	return r.prefix + "obj:" + hex.EncodeToString(h[:])
}

func (r *RedisRepo) activeSet() string { return r.prefix + "active" }

func (r *RedisRepo) UpsertOnFetch(url, userID string, fetched FetchResult, now time.Time) (int64, PaymentRecord, error) {
	key := r.keyFor(url)

	// Get existing
	bs, err := r.c.Get(context.Background(), key).Bytes()
	if err != nil && err != redis.Nil {
		return 0, PaymentRecord{}, err
	}
	if err == redis.Nil {
		rec := PaymentRecord{
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
		out, _ := json.Marshal(rec)
		if err := r.c.Set(context.Background(), key, out, 0).Err(); err != nil {
			return 0, PaymentRecord{}, err
		}
		// add to active set (use URL as member)
		if err := r.c.SAdd(context.Background(), r.activeSet(), url).Err(); err != nil {
			return 0, PaymentRecord{}, err
		}
		return fetched.TotalMinor, rec, nil
	}

	var rec PaymentRecord
	if err := json.Unmarshal(bs, &rec); err != nil {
		return 0, PaymentRecord{}, err
	}

	// Reactivate and set userID if empty
	rec.Active = true
	if rec.UserID == "" {
		rec.UserID = userID
	}
	if rec.AssetCode == "" {
		rec.AssetCode = fetched.AssetCode
	}
	if rec.AssetScale == 0 {
		rec.AssetScale = fetched.AssetScale
	}

	delta := fetched.TotalMinor - rec.LastValue
	if delta < 0 {
		delta = 0
	}
	if fetched.TotalMinor > rec.LastValue {
		rec.LastChangeAt = now
		rec.LastValue = fetched.TotalMinor
	}
	rec.LastUpdatedAt = now

	out, _ := json.Marshal(rec)
	if err := r.c.Set(context.Background(), key, out, 0).Err(); err != nil {
		return 0, PaymentRecord{}, err
	}
	if err := r.c.SAdd(context.Background(), r.activeSet(), url).Err(); err != nil {
		return 0, PaymentRecord{}, err
	}
	return delta, rec, nil
}

func (r *RedisRepo) Get(url string) (PaymentRecord, bool, error) {
	key := r.keyFor(url)
	bs, err := r.c.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return PaymentRecord{}, false, nil
	}
	if err != nil {
		return PaymentRecord{}, false, err
	}
	var rec PaymentRecord
	if err := json.Unmarshal(bs, &rec); err != nil {
		return PaymentRecord{}, false, err
	}
	return rec, true, nil
}

func (r *RedisRepo) ListActive() ([]PaymentRecord, error) {
	members, err := r.c.SMembers(context.Background(), r.activeSet()).Result()
	if err != nil {
		return nil, err
	}
	out := make([]PaymentRecord, 0, len(members))
	for _, url := range members {
		rec, ok, err := r.Get(url)
		if err != nil {
			return nil, err
		}
		if ok && rec.Active {
			out = append(out, rec)
		} else if ok && !rec.Active {
			// clean up set
			_ = r.c.SRem(context.Background(), r.activeSet(), url).Err()
		}
	}
	return out, nil
}

func (r *RedisRepo) MarkActive(url string, now time.Time) error {
	rec, ok, err := r.Get(url)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not found")
	}
	rec.Active = true
	rec.LastUpdatedAt = now
	out, _ := json.Marshal(rec)
	if err := r.c.Set(context.Background(), r.keyFor(url), out, 0).Err(); err != nil {
		return err
	}
	return r.c.SAdd(context.Background(), r.activeSet(), url).Err()
}

func (r *RedisRepo) MarkInactive(url string, now time.Time) error {
	rec, ok, err := r.Get(url)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not found")
	}
	rec.Active = false
	rec.LastUpdatedAt = now
	out, _ := json.Marshal(rec)
	if err := r.c.Set(context.Background(), r.keyFor(url), out, 0).Err(); err != nil {
		return err
	}
	return r.c.SRem(context.Background(), r.activeSet(), url).Err()
}
