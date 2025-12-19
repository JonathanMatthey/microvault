package incoming

import "time"

// PaymentRecord is the persisted state for an incoming payment URL.
type PaymentRecord struct {
	URL           string    `json:"url"`
	UserID        string    `json:"userId"`
	AssetCode     string    `json:"assetCode"`
	AssetScale    int       `json:"assetScale"`
	LastValue     int64     `json:"lastValue"` // minor units
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	LastChangeAt  time.Time `json:"lastChangeAt"`
	Active        bool      `json:"active"`
}

// FetchResult represents a parsed incoming-payment fetch.
type FetchResult struct {
	TotalMinor int64
	AssetCode  string
	AssetScale int
	RawJSON    []byte
}

// Repository abstracts persistence for PaymentRecord entries.
type Repository interface {
	// UpsertOnFetch creates or updates the record based on a fetch result.
	// It returns the positive delta (increase in TotalMinor) if any.
	UpsertOnFetch(url, userID string, fetched FetchResult, now time.Time) (delta int64, updated PaymentRecord, err error)

	// Get returns the record for a URL.
	Get(url string) (PaymentRecord, bool, error)

	// ListActive returns all currently active records.
	ListActive() ([]PaymentRecord, error)

	// MarkActive marks a record active.
	MarkActive(url string, now time.Time) error

	// MarkInactive marks a record inactive.
	MarkInactive(url string, now time.Time) error
}
