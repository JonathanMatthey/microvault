package identity

import (
	"context"
	"errors"
	"time"
)

var ErrMissingDevUser = errors.New("devlogin: missing user id in context")

// DevLoginProvider returns a user based on context for local development.
type DevLoginProvider struct {
	Now func() time.Time
}

func (p *DevLoginProvider) Authenticate(ctx context.Context) (User, error) {
	val := ctx.Value(UserIDKey())
	userID, ok := val.(string)
	if !ok || userID == "" {
		return User{}, ErrMissingDevUser
	}

	now := time.Now()
	if p != nil && p.Now != nil {
		now = p.Now()
	}

	return User{
		ID:         userID,
		Hash:       HashUserID(userID),
		LastActive: now.Unix(),
	}, nil
}
