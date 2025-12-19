package incoming

import (
	"testing"
	"time"
)

func TestMemoryRepo_UpsertOnFetch_CreateAndUpdate(t *testing.T) {
	repo := NewMemoryRepo()
	now := time.Now()
	url := "https://example.test/incoming/123"
	user := "user-1"

	// Create with zero value
	d, rec, err := repo.UpsertOnFetch(url, user, FetchResult{TotalMinor: 0, AssetCode: "USD", AssetScale: 2}, now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != 0 {
		t.Fatalf("delta want 0 got %d", d)
	}
	if !rec.Active {
		t.Fatalf("expected active")
	}
	if !rec.CreatedAt.Equal(now) {
		t.Fatalf("createdAt not set")
	}
	if !rec.LastUpdatedAt.Equal(now) {
		t.Fatalf("lastUpdatedAt not set")
	}
	if !rec.LastChangeAt.IsZero() {
		t.Fatalf("lastChangeAt should be zero when value=0")
	}

	// Increase to 5
	later := now.Add(1 * time.Minute)
	d, rec, err = repo.UpsertOnFetch(url, user, FetchResult{TotalMinor: 5, AssetCode: "USD", AssetScale: 2}, later)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != 5 {
		t.Fatalf("delta want 5 got %d", d)
	}
	if rec.LastValue != 5 {
		t.Fatalf("lastValue want 5 got %d", rec.LastValue)
	}
	if !rec.LastChangeAt.Equal(later) {
		t.Fatalf("lastChangeAt not updated")
	}

	// No change -> delta 0, don't move LastChangeAt
	evenLater := later.Add(1 * time.Minute)
	d, rec, err = repo.UpsertOnFetch(url, user, FetchResult{TotalMinor: 5, AssetCode: "USD", AssetScale: 2}, evenLater)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != 0 {
		t.Fatalf("delta want 0 got %d", d)
	}
	if !rec.LastChangeAt.Equal(later) {
		t.Fatalf("lastChangeAt should remain at increase time")
	}

	// Reactivate from inactive
	if err := repo.MarkInactive(url, evenLater); err != nil {
		t.Fatalf("mark inactive: %v", err)
	}
	d, rec, err = repo.UpsertOnFetch(url, user, FetchResult{TotalMinor: 6, AssetCode: "USD", AssetScale: 2}, evenLater.Add(1*time.Minute))
	if !rec.Active {
		t.Fatalf("expected active after upsert")
	}
	if d != 1 {
		t.Fatalf("delta want 1 got %d", d)
	}
}
