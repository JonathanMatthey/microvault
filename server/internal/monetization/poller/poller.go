package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bosbaber/hackweek/microvault/internal/ledger"
	"github.com/bosbaber/hackweek/microvault/internal/monetization/store"
)

const (
	PollInterval    = 3 * time.Second
	MaxIdleDuration = 2 * time.Hour
)

type IncomingPaymentResponse struct {
	ReceivedAmount struct {
		Value      string `json:"value"`
		Amount     string `json:"amount"`
		AssetCode  string `json:"assetCode"`
		AssetScale int    `json:"assetScale"`
	} `json:"receivedAmount"`
}

type Poller struct {
	httpClient     *http.Client
	authToken      string
	ledger         ledger.Ledger
	store          store.Store
	unitsPerCredit int64
	creditUnit     int64
	expectedAsset  string
	expectedScale  int
	mu             sync.Mutex
	activePolls    map[string]context.CancelFunc
}

func New(
	httpClient *http.Client,
	authToken string,
	ledger ledger.Ledger,
	store store.Store,
	unitsPerCredit int64,
	creditUnit int64,
	expectedAsset string,
	expectedScale int,
) *Poller {
	return &Poller{
		httpClient:     httpClient,
		authToken:      authToken,
		ledger:         ledger,
		store:          store,
		unitsPerCredit: unitsPerCredit,
		creditUnit:     creditUnit,
		expectedAsset:  expectedAsset,
		expectedScale:  expectedScale,
		activePolls:    make(map[string]context.CancelFunc),
	}
}

func (p *Poller) StartPolling(userID, incomingPaymentURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := userID + ":" + incomingPaymentURL
	if _, exists := p.activePolls[key]; exists {
		log.Printf("[wm-poller] already polling user=%s incoming=%s", userID, incomingPaymentURL)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.activePolls[key] = cancel

	go p.pollLoop(ctx, userID, incomingPaymentURL, key)
	log.Printf("[wm-poller] started polling user=%s incoming=%s", userID, incomingPaymentURL)
}

func (p *Poller) pollLoop(ctx context.Context, userID, incomingPaymentURL, key string) {
	defer func() {
		p.mu.Lock()
		delete(p.activePolls, key)
		p.mu.Unlock()
		log.Printf("[wm-poller] stopped polling user=%s incoming=%s", userID, incomingPaymentURL)
	}()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	lastChange := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := p.checkAndCredit(ctx, userID, incomingPaymentURL)
			if err != nil {
				log.Printf("[wm-poller] check failed user=%s incoming=%s error=%v", userID, incomingPaymentURL, err)
				continue
			}

			if changed {
				lastChange = time.Now()
			}

			if time.Since(lastChange) > MaxIdleDuration {
				log.Printf("[wm-poller] idle timeout user=%s incoming=%s", userID, incomingPaymentURL)
				return
			}
		}
	}
}

func (p *Poller) checkAndCredit(ctx context.Context, userID, incomingPaymentURL string) (bool, error) {
	// Fetch the incoming payment
	req, err := http.NewRequestWithContext(ctx, "GET", incomingPaymentURL, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if p.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.authToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("fetch incoming payment: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ipResp IncomingPaymentResponse
	if err := json.Unmarshal(body, &ipResp); err != nil {
		log.Printf("[wm-poller] parse failed user=%s incoming=%s body=%s", userID, incomingPaymentURL, string(body))
		return false, fmt.Errorf("parse json: %w", err)
	}

	// Extract amount (try "amount" first, then "value")
	amountStr := ipResp.ReceivedAmount.Amount
	if amountStr == "" {
		amountStr = ipResp.ReceivedAmount.Value
	}
	if amountStr == "" {
		log.Printf("[wm-poller] ZERO VALUE user=%s incoming=%s body=%s", userID, incomingPaymentURL, string(body))
		return false, nil
	}

	totalMinor, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse amount %q: %w", amountStr, err)
	}

	if totalMinor == 0 {
		log.Printf("[wm-poller] ZERO VALUE user=%s incoming=%s body=%s", userID, incomingPaymentURL, string(body))
		return false, nil
	}

	// Validate asset if configured
	if p.expectedAsset != "" && ipResp.ReceivedAmount.AssetCode != p.expectedAsset {
		return false, fmt.Errorf("asset code mismatch: got=%s want=%s", ipResp.ReceivedAmount.AssetCode, p.expectedAsset)
	}
	if p.expectedScale > 0 && ipResp.ReceivedAmount.AssetScale != p.expectedScale {
		return false, fmt.Errorf("asset scale mismatch: got=%d want=%d", ipResp.ReceivedAmount.AssetScale, p.expectedScale)
	}

	// Get last verified amount
	lastMinor, err := p.store.GetLastAmount(ctx, userID, incomingPaymentURL)
	if err != nil {
		return false, fmt.Errorf("get last amount: %w", err)
	}

	deltaMinor := totalMinor - lastMinor
	if deltaMinor <= 0 {
		return false, nil // No increase
	}

	// Convert to credits and credit user
	credited := (deltaMinor * p.creditUnit) / p.unitsPerCredit
	if credited <= 0 {
		// Delta too small to credit, but still update last amount
		if err := p.store.SetLastAmount(ctx, userID, incomingPaymentURL, totalMinor); err != nil {
			return false, fmt.Errorf("update last amount: %w", err)
		}
		return true, nil
	}

	if err := p.ledger.Credit(userID, credited); err != nil {
		return false, fmt.Errorf("credit ledger: %w", err)
	}

	if err := p.store.SetLastAmount(ctx, userID, incomingPaymentURL, totalMinor); err != nil {
		return false, fmt.Errorf("update last amount: %w", err)
	}

	balance := p.ledger.Balance(userID)
	log.Printf("[wm-poller] credited user=%s incoming=%s delta_units=%d credited=%d new_balance=%d",
		userID, incomingPaymentURL, deltaMinor, credited, balance)

	return true, nil
}

func (p *Poller) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, cancel := range p.activePolls {
		cancel()
		log.Printf("[wm-poller] stopping %s", key)
	}
	p.activePolls = make(map[string]context.CancelFunc)
}
