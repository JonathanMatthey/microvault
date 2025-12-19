package activity

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bosbaber/hackweek/microvault/internal/core"
	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
	"github.com/google/uuid"
)

// StorageCharger periodically charges users for stored data based on a configured interval rate.
type StorageCharger struct {
	ledger               ledger.Ledger
	s3Client             *storage.S3Client
	interval             time.Duration
	chargePerGiBInterval int64
	stopCh               chan struct{}
}

// NewStorageCharger constructs a charger.
func NewStorageCharger(l ledger.Ledger, s3 *storage.S3Client, chargeFrequencyMinutes int64, chargePerGiBInterval int64) *StorageCharger {
	if chargeFrequencyMinutes <= 0 {
		chargeFrequencyMinutes = 60
	}
	return &StorageCharger{
		ledger:               l,
		s3Client:             s3,
		interval:             time.Duration(chargeFrequencyMinutes) * time.Minute,
		chargePerGiBInterval: chargePerGiBInterval,
		stopCh:               make(chan struct{}),
	}
}

// Start begins the periodic charging loop.
func (sc *StorageCharger) Start() {
	if sc.chargePerGiBInterval <= 0 {
		log.Printf("[storage-charge] skipped start: interval rate is zero")
		return
	}
	if sc.s3Client == nil {
		log.Printf("[storage-charge] skipped start: s3 client is nil")
		return
	}
	ticker := time.NewTicker(sc.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-sc.stopCh:
				return
			case <-ticker.C:
				sc.chargeAll()
			}
		}
	}()
	log.Printf("[storage-charge] started: interval=%s rate_per_gib=%d", sc.interval, sc.chargePerGiBInterval)
}

// Stop stops the periodic charging loop.
func (sc *StorageCharger) Stop() {
	close(sc.stopCh)
}

// chargeAll charges every user based on their current stored bytes.
func (sc *StorageCharger) chargeAll() {
	ctx := context.Background()
	users, err := sc.ledger.ListAll()
	if err != nil {
		log.Printf("[storage-charge] list users failed: %v", err)
		return
	}
	for userID := range users {
		sc.chargeUser(ctx, userID)
	}
}

func (sc *StorageCharger) chargeUser(ctx context.Context, userID string) {
	userHash := storage.UserHashFromID(userID)
	files, err := sc.s3Client.ListUserFiles(ctx, userHash)
	if err != nil {
		log.Printf("[storage-charge] list files failed user=%s err=%v", userID, err)
		return
	}

	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
	}
	if totalBytes <= 0 {
		return
	}
	cost := core.CalculateStorageIntervalCost(totalBytes, sc.chargePerGiBInterval)
	if cost <= 0 {
		return
	}
	if err := sc.ledger.Debit(userID, cost); err != nil {
		log.Printf("[storage-charge] debit failed user=%s cost=%d err=%v", userID, cost, err)
		return
	}
	balance := sc.ledger.Balance(userID)
	_ = sc.ledger.RecordTransaction(userID, ledger.Transaction{
		ID:           uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Type:         "debit",
		Reason:       "storage",
		Amount:       -cost,
		BalanceAfter: balance,
		Description:  fmt.Sprintf("Storage charge for %d bytes", totalBytes),
		Metadata: map[string]interface{}{
			"bytes":    totalBytes,
			"gib_rate": sc.chargePerGiBInterval,
		},
	})
	log.Printf("[storage-charge] charged user=%s bytes=%d cost=%d balance=%d", userID, totalBytes, cost, balance)
}
