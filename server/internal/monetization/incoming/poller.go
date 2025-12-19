package incoming

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/bosbaber/hackweek/selfstack/internal/ledger"
	"github.com/google/uuid"
)

type Poller struct {
	repo           Repository
	httpClient     *http.Client
	authToken      string
	unitsPerCredit int64
	creditUnit     int64
	expectedAsset  string
	expectedScale  int
	pollInterval   time.Duration
	inactiveAfter  time.Duration
	stopCh         chan struct{}
}

func NewPoller(repo Repository, httpClient *http.Client, authToken string, unitsPerCredit int64, creditUnit int64, expectedAsset string, expectedScale int) *Poller {
	return &Poller{
		repo:           repo,
		httpClient:     httpClient,
		authToken:      authToken,
		unitsPerCredit: unitsPerCredit,
		creditUnit:     creditUnit,
		expectedAsset:  expectedAsset,
		expectedScale:  expectedScale,
		pollInterval:   10 * time.Second,
		inactiveAfter:  24 * time.Hour,
		stopCh:         make(chan struct{}),
	}
}

// For tests
func (p *Poller) SetIntervals(poll, inactive time.Duration) {
	p.pollInterval, p.inactiveAfter = poll, inactive
}

func (p *Poller) Start(l ledger.Ledger) {
	ticker := time.NewTicker(p.pollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.tick(l)
			}
		}
	}()
}

func (p *Poller) Stop() { close(p.stopCh) }

func (p *Poller) tick(l ledger.Ledger) {
	records, err := p.repo.ListActive()
	if err != nil {
		log.Printf("[wm-incoming] list active failed: %v", err)
		return
	}
	for _, rec := range records {
		// Fetch
		fr, err := p.fetch(rec.URL)
		if err != nil {
			log.Printf("[wm-incoming] fetch %s failed: %v", rec.URL, err)
			continue
		}
		now := time.Now()
		// Validate asset if configured
		if p.expectedAsset != "" && fr.AssetCode != p.expectedAsset {
			log.Printf("[wm-incoming] asset code mismatch url=%s got=%s want=%s", rec.URL, fr.AssetCode, p.expectedAsset)
			continue
		}
		if p.expectedScale > 0 && fr.AssetScale != p.expectedScale {
			log.Printf("[wm-incoming] asset scale mismatch url=%s got=%d want=%d", rec.URL, fr.AssetScale, p.expectedScale)
			continue
		}
		delta, updated, err := p.repo.UpsertOnFetch(rec.URL, rec.UserID, fr, now)
		if err != nil {
			log.Printf("[wm-incoming] upsert failed url=%s err=%v", rec.URL, err)
			continue
		}
		if delta > 0 {
			// Convert currency minor units to internal credit units
			// delta = currency minor units received
			// unitsPerCredit = currency minor units required for 1 credit (price per credit)
			// creditUnit = 10^creditScale = internal representation multiplier for decimal precision
			// Formula: credited = (delta / unitsPerCredit) * creditUnit
			// Example: received 123 minor units ($1.23), unitsPerCredit=10 (10 minor = 1 credit), creditUnit=10000
			//   credited = (123 / 10) * 10000 = 12.3 * 10000 = 123000 internal units
			credited := (delta * p.creditUnit) / p.unitsPerCredit
			if credited > 0 {
				if err := l.Credit(updated.UserID, credited); err != nil {
					log.Printf("[wm-incoming] credit failed user=%s err=%v", updated.UserID, err)
				} else {
					newBalanceRaw := l.Balance(updated.UserID)
					// Bright green log for successful credits
					log.Printf("\033[92m[wm-incoming] credited user=%s url=%s delta_units=%d credited_raw=%d new_balance_raw=%d new_balance_display=%.4f\033[0m",
						updated.UserID, rec.URL, delta, credited, newBalanceRaw, float64(newBalanceRaw)/float64(p.creditUnit))
					_ = l.RecordTransaction(updated.UserID, ledger.Transaction{
						ID:           uuid.NewString(),
						Timestamp:    time.Now().UTC(),
						Type:         "credit",
						Reason:       "payment",
						Amount:       credited,
						BalanceAfter: newBalanceRaw,
						Description:  "Incoming Web Monetization payment",
						Metadata:     map[string]interface{}{"asset": fr.AssetCode, "scale": fr.AssetScale},
					})
				}
			}
		}
		// Inactivity check
		if !updated.LastChangeAt.IsZero() && time.Since(updated.LastChangeAt) > p.inactiveAfter {
			if err := p.repo.MarkInactive(rec.URL, now); err == nil {
				log.Printf("[wm-incoming] marked inactive url=%s", rec.URL)
			}
		}
	}
}

func (p *Poller) fetch(url string) (FetchResult, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	if p.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.authToken)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return FetchResult{}, fmt.Errorf("status %s: %s", resp.Status, string(b))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return FetchResult{}, err
	}
	var payload struct {
		ReceivedAmount struct {
			Amount     string `json:"amount"`
			Value      string `json:"value"`
			AssetCode  string `json:"assetCode"`
			AssetScale int    `json:"assetScale"`
		} `json:"receivedAmount"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[wm-incoming] parse failed url=%s body=%s", url, string(body))
		return FetchResult{}, err
	}
	amtStr := payload.ReceivedAmount.Amount
	if amtStr == "" {
		amtStr = payload.ReceivedAmount.Value
	}
	if amtStr == "" {
		log.Printf("[wm-incoming] ZERO VALUE url=%s body=%s", url, string(body))
		return FetchResult{TotalMinor: 0, AssetCode: payload.ReceivedAmount.AssetCode, AssetScale: payload.ReceivedAmount.AssetScale, RawJSON: body}, nil
	}
	totalMinor, err := strconv.ParseInt(amtStr, 10, 64)
	if err != nil {
		return FetchResult{}, fmt.Errorf("invalid amount %q: %w", amtStr, err)
	}
	return FetchResult{TotalMinor: totalMinor, AssetCode: payload.ReceivedAmount.AssetCode, AssetScale: payload.ReceivedAmount.AssetScale, RawJSON: body}, nil
}
