package incoming

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	memledger "github.com/bosbaber/hackweek/selfstack/internal/ledger/memory"
)

func TestPoller_CreditsDeltas(t *testing.T) {
	repo := NewMemoryRepo()
	ledger := memledger.New()

	// Increasing from 0 by 2 up to 6 (then stays)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
	}))
	defer srv.Close()

	vals := []int64{0, 2, 4, 6, 6}
	idx := 0
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if idx < len(vals)-1 {
			idx++
		}
		v := vals[idx]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"receivedAmount":{"value":"` + fmt.Sprintf("%d", v) + `","assetCode":"USD","assetScale":2}}`,
		))
	})

	url := srv.URL

	// Test config: unitsPerCredit=2 (2 minor currency units per 1 credit), creditUnit=10 (scale=1)
	creditUnit := int64(10)
	poller := NewPoller(repo, srv.Client(), "", 2, creditUnit, "USD", 2)
	poller.SetIntervals(50*time.Millisecond, 24*time.Hour)

	// Seed record on first event (value will be 2 on first fetch by poller)
	if _, _, err := repo.UpsertOnFetch(url, "user-1", FetchResult{TotalMinor: 0, AssetCode: "USD", AssetScale: 2}, time.Now()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	poller.Start(ledger)
	defer poller.Stop()

	// Wait a bit for a few ticks
	time.Sleep(300 * time.Millisecond)

	bal := ledger.Balance("user-1")
	// Final value is 6, initial was 0 -> delta 6 minor units
	// credited units = (delta / unitsPerCredit) * creditUnit = (6 / 2) * 10 = 30
	if bal != 30 {
		t.Fatalf("balance want 30 got %d", bal)
	}
}
