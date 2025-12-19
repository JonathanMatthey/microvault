package identity

import "context"

// User represents an authenticated user identity.
type User struct {
	ID         string
	Hash       string
	LastActive int64 // unix seconds for easy determinism in tests
}

// IdentityProvider authenticates a user from the current context.
type IdentityProvider interface {
	Authenticate(ctx context.Context) (User, error)
}

// Context key for injecting user ID.
type ctxKey string

const userIDKey ctxKey = "user_id"

// WithUserID annotates a context with a user ID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDKey returns the context key used for storing user IDs.
func UserIDKey() interface{} {
	return userIDKey
}
