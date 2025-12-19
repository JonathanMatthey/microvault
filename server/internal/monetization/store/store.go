package store

import "context"

// Store tracks last verified amounts per user/incomingPayment URL
type Store interface {
	GetLastAmount(ctx context.Context, userID, incomingPayment string) (int64, error)
	SetLastAmount(ctx context.Context, userID, incomingPayment string, amount int64) error
}
