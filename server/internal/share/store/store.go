package store

import "time"

// Link represents a share link record
type Link struct {
	Token     string    `json:"token"`
	OwnerID   string    `json:"ownerId"`
	UploadID  string    `json:"uploadId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Downloads int64     `json:"downloads"`
}

// Store is the interface for managing share links
type Store interface {
	// Create creates a new share link for the given owner/upload with TTL.
	Create(ownerID, uploadID string, ttl time.Duration) (Link, error)
	// Get returns a link by token if it exists and is not expired.
	Get(token string) (Link, bool, error)
	// IncrementDownloads increments the download counter for the link.
	IncrementDownloads(token string) error
}
