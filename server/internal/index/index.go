package index

import (
	"context"
	"os"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// FileRecord is a normalized metadata record for an uploaded file.
// Times are RFC3339 strings to keep Redis storage simple.
type FileRecord struct {
	ID         string
	UserID     string
	Filename   string
	Size       int64
	CreatedAt  string // RFC3339
	UploadedAt string // RFC3339
}

// Indexer provides methods to manage file indices for quick listing.
type Indexer interface {
	AddFile(ctx context.Context, rec FileRecord) error
	ListFiles(ctx context.Context, userID string, limit int64) ([]FileRecord, error)
	RemoveFile(ctx context.Context, userID string, uploadID string) error
	GetFile(ctx context.Context, uploadID string) (*FileRecord, error)
}

// RedisIndexer implements Indexer backed by Redis.
// Keys used:
// - sorted set: user:{userID}:files with member=uploadID, score=uploadedAt epoch seconds
// - hash: file:{uploadID} with fields user, filename, size, createdAt, uploadedAt
type RedisIndexer struct {
	rdb *redis.Client
}

// NewRedisIndexerFromEnv initializes a Redis client using MICROVAULT_REDIS_ADDR env var.
// If MICROVAULT_REDIS_ADDR is empty, defaults to localhost:6379.
func NewRedisIndexerFromEnv() (*RedisIndexer, error) {
	addr := os.Getenv("MICROVAULT_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisIndexer{rdb: rdb}, nil
}

// NewRedisIndexer creates a Redis indexer with explicit connection options.
func NewRedisIndexer(addr string, db int, password string) (*RedisIndexer, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db, Password: password})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisIndexer{rdb: rdb}, nil
}

func (r *RedisIndexer) AddFile(ctx context.Context, rec FileRecord) error {
	if rec.ID == "" || rec.UserID == "" {
		return nil
	}
	hKey := "file:" + rec.ID
	zKey := "user:" + rec.UserID + ":files"
	uploadedEpoch := time.Now().Unix()
	if rec.UploadedAt != "" {
		if t, err := time.Parse(time.RFC3339, rec.UploadedAt); err == nil {
			uploadedEpoch = t.Unix()
		}
	}
	// Write hash
	if err := r.rdb.HSet(ctx, hKey, map[string]any{
		"user":       rec.UserID,
		"filename":   rec.Filename,
		"size":       rec.Size,
		"createdAt":  rec.CreatedAt,
		"uploadedAt": rec.UploadedAt,
	}).Err(); err != nil {
		return err
	}
	// Add to sorted set
	return r.rdb.ZAdd(ctx, zKey, redis.Z{Score: float64(uploadedEpoch), Member: rec.ID}).Err()
}

func (r *RedisIndexer) ListFiles(ctx context.Context, userID string, limit int64) ([]FileRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	zKey := "user:" + userID + ":files"
	ids, err := r.rdb.ZRevRange(ctx, zKey, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	records := make([]FileRecord, 0, len(ids))
	for _, id := range ids {
		hKey := "file:" + id
		m, err := r.rdb.HGetAll(ctx, hKey).Result()
		if err != nil || len(m) == 0 {
			continue
		}
		size, _ := strconv.ParseInt(m["size"], 10, 64)
		rec := FileRecord{
			ID:         id,
			UserID:     m["user"],
			Filename:   m["filename"],
			Size:       size,
			CreatedAt:  m["createdAt"],
			UploadedAt: m["uploadedAt"],
		}
		records = append(records, rec)
	}
	return records, nil
}

func (r *RedisIndexer) RemoveFile(ctx context.Context, userID string, uploadID string) error {
	zKey := "user:" + userID + ":files"
	hKey := "file:" + uploadID
	if err := r.rdb.ZRem(ctx, zKey, uploadID).Err(); err != nil {
		return err
	}
	return r.rdb.Del(ctx, hKey).Err()
}

func (r *RedisIndexer) GetFile(ctx context.Context, uploadID string) (*FileRecord, error) {
	hKey := "file:" + uploadID
	m, err := r.rdb.HGetAll(ctx, hKey).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	size, _ := strconv.ParseInt(m["size"], 10, 64)
	rec := &FileRecord{
		ID:         uploadID,
		UserID:     m["user"],
		Filename:   m["filename"],
		Size:       size,
		CreatedAt:  m["createdAt"],
		UploadedAt: m["uploadedAt"],
	}
	return rec, nil
}
