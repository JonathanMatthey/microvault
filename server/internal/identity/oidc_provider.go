package identity

import (
	"context"
	"errors"
	"time"
)

// OIDCProvider authenticates a user for production by reading the user ID
// previously injected into the context by an upstream auth middleware.
type OIDCProvider struct {
	Now func() time.Time
}

func (p *OIDCProvider) Authenticate(ctx context.Context) (User, error) {
	val := ctx.Value(UserIDKey())
	userID, ok := val.(string)
	if !ok || userID == "" {
		return User{}, errors.New("oidc: missing user id in context")
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
