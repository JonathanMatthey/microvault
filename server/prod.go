package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/bosbaber/hackweek/selfstack/internal/account"
	"github.com/bosbaber/hackweek/selfstack/internal/activity"
	"github.com/bosbaber/hackweek/selfstack/internal/core"
	"github.com/bosbaber/hackweek/selfstack/internal/ledger"
	"github.com/bosbaber/hackweek/selfstack/internal/ledger/memory"
	redisledger "github.com/bosbaber/hackweek/selfstack/internal/ledger/redis"
	"github.com/bosbaber/hackweek/selfstack/internal/monetization/incoming"
	"github.com/bosbaber/hackweek/selfstack/internal/policy"
	"github.com/bosbaber/hackweek/selfstack/internal/share"
	sharestore "github.com/bosbaber/hackweek/selfstack/internal/share/store"
	"github.com/bosbaber/hackweek/selfstack/internal/storage"
	"github.com/bosbaber/hackweek/selfstack/internal/upload"
)

// fetchWalletMetadata fetches the Open Payments wallet address metadata to get assetCode and assetScale
func fetchWalletMetadata(paymentPointer string) (assetCode string, assetScale int, err error) {
	// Convert payment pointer ($host/path) to HTTPS URL
	walletURL := paymentPointer
	if len(paymentPointer) > 0 && paymentPointer[0] == '$' {
		walletURL = "https://" + paymentPointer[1:]
	}

	resp, err := http.Get(walletURL)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch wallet metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("wallet metadata fetch returned status %d", resp.StatusCode)
	}

	var metadata struct {
		AssetCode  string `json:"assetCode"`
		AssetScale int    `json:"assetScale"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", 0, fmt.Errorf("failed to decode wallet metadata: %w", err)
	}

	if metadata.AssetCode == "" {
		return "", 0, fmt.Errorf("wallet metadata missing assetCode")
	}

	return metadata.AssetCode, metadata.AssetScale, nil
}

func initProdMode(e *echo.Echo, cfg *Config) {
	// In Phase 5, production also uses S3 (no TUS)
	log.Printf("S3 bucket: %s (region: %s)", cfg.S3.Bucket, cfg.S3.Region)

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(storage.S3Config{
		Endpoint:                 cfg.S3.Endpoint,
		Region:                   cfg.S3.Region,
		Bucket:                   cfg.S3.Bucket,
		AccessKey:                cfg.S3.AccessKey,
		SecretKey:                cfg.S3.SecretKey,
		PresignedURLExpiryUpload: cfg.S3.PresignedURLExpiryUpload,
		PresignedURLExpiryDL:     cfg.S3.PresignedURLExpiryDL,
	})
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}
	log.Printf("S3 client initialized successfully")

	// Initialize Google Auth if enabled
	if cfg.Auth.Enabled {
		if err := InitGoogleAuth(); err != nil {
			log.Fatalf("Failed to initialize Google Auth: %v", err)
		}
		log.Printf("Google Auth initialized successfully")
	}

	// Enable CORS for frontend
	e.Use(middleware.CORSWithConfig(buildCORSConfig(cfg)))

	// Initialize core services
	var l ledger.Ledger
	if cfg.Server.Redis.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Redis.Host, cfg.Server.Redis.Port)
		redisLedger, err := redisledger.New(addr, cfg.Server.Redis.DB, cfg.Server.Redis.Password, cfg.Server.Redis.KeyPrefix)
		if err != nil {
			log.Fatalf("Failed to create Redis ledger: %v", err)
		}
		l = redisLedger
		log.Printf("Using Redis ledger at %s (db=%d, prefix=%s)", addr, cfg.Server.Redis.DB, cfg.Server.Redis.KeyPrefix)
	} else {
		l = memory.New()
		log.Printf("Using in-memory ledger (not persistent!)")
	}

	// Create policy engine with minimum cost of 1 byte at ingress rate
	minCost := core.CalculateIngressCost(1, cfg.Credits.IngressPerGiB)
	if minCost == 0 {
		minCost = 1 // At least 1 credit unit required
	}
	p := policy.New(minCost)
	act := activity.New()

	a := &app{
		ledger:   l,
		policy:   p,
		activity: act,
	}

	// Start periodic storage charging
	chargePerInterval := cfg.Credits.StorageChargePerGiBInterval()
	charger := activity.NewStorageCharger(l, s3Client, cfg.Credits.ChargeFrequencyMin, chargePerInterval)
	charger.Start()

	// Fetch wallet metadata for monetization if enabled
	var walletAssetCode string
	var walletAssetScale int
	if cfg.Monetization.Enabled {
		code, scale, err := fetchWalletMetadata(cfg.Monetization.PaymentPointer)
		if err != nil {
			log.Fatalf("Failed to fetch wallet metadata from %s: %v", cfg.Monetization.PaymentPointer, err)
		}
		walletAssetCode = code
		walletAssetScale = scale
		log.Printf("Wallet metadata: assetCode=%s, assetScale=%d", walletAssetCode, walletAssetScale)
	}

	// Health check endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok", "mode": "prod"})
	})

	// Initialize monetization poller if enabled
	var incomingRepo incoming.Repository
	if cfg.Monetization.Enabled && cfg.Server.Redis.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Redis.Host, cfg.Server.Redis.Port)
		repo, err := incoming.NewRedisRepo(addr, cfg.Server.Redis.DB, cfg.Server.Redis.Password, "selfstack:ip:")
		if err != nil {
			log.Fatalf("Failed to create incoming payments repository: %v", err)
		}
		incomingRepo = repo

		// Start the poller with wallet's actual asset info
		creditUnit := creditUnitFromScale(cfg.Credits.Scale)
		poller := incoming.NewPoller(
			repo,
			http.DefaultClient,
			cfg.Monetization.IncomingAuthToken,
			cfg.Credits.UnitsPerCredit,
			creditUnit,
			walletAssetCode,
			walletAssetScale,
		)
		poller.Start(l)
		log.Printf("Web Monetization poller started (payment_pointer=%s, units_per_credit=%d, asset=%s, scale=%d)",
			cfg.Monetization.PaymentPointer, cfg.Credits.UnitsPerCredit, walletAssetCode, walletAssetScale)
	} else {
		// Use in-memory repository as fallback
		incomingRepo = incoming.NewMemoryRepo()
		log.Printf("Web Monetization disabled or Redis unavailable - using memory repository")
	}

	// Monetization config endpoint
	e.GET("/monetization/config", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled":          cfg.Monetization.Enabled,
			"payment_pointer":  cfg.Monetization.PaymentPointer,
			"asset_code":       walletAssetCode,
			"asset_scale":      walletAssetScale,
			"units_per_credit": cfg.Credits.UnitsPerCredit,
			"min_topup_units":  cfg.Monetization.MinTopupUnits,
			"max_topup_units":  cfg.Monetization.MaxTopupUnits,
		})
	})

	// Monetization verify endpoint - registers the incoming payment URL for polling
	e.POST("/monetization/verify", a.withUser(func(c echo.Context) error {
		userID := c.Get("userID").(string)
		var req struct {
			ReceiptURL string `json:"receipt_url"`
		}
		if err := c.Bind(&req); err != nil || req.ReceiptURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "receipt_url required"})
		}

		// Check if already registered, if not create an initial stub record
		_, exists, err := incomingRepo.Get(req.ReceiptURL)
		if err != nil {
			log.Printf("Failed to check receipt URL: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check receipt"})
		}

		if !exists {
			// Create a stub record with zero value and wallet's asset info - the poller will update it on first fetch
			stub := incoming.FetchResult{
				TotalMinor: 0,
				AssetCode:  walletAssetCode,
				AssetScale: walletAssetScale,
			}

			_, _, err = incomingRepo.UpsertOnFetch(req.ReceiptURL, userID, stub, time.Now())
			if err != nil {
				log.Printf("Failed to create initial record: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create record"})
			}
			log.Printf("Created new payment record: user=%s url=%s asset=%s scale=%d", userID, req.ReceiptURL, walletAssetCode, walletAssetScale)
		} else {
			// Just mark as active
			if err := incomingRepo.MarkActive(req.ReceiptURL, time.Now()); err != nil {
				log.Printf("Failed to mark receipt URL active: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to activate"})
			}
			log.Printf("Reactivated existing payment record: user=%s url=%s", userID, req.ReceiptURL)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"credits": a.ledger.Balance(userID),
		})
	}))

	// Account management (production: X-User-ID auth like dev)
	// TODO: In future phases, replace with Google OAuth
	e.POST("/login", a.handleLogin)
	e.GET("/balance", a.withUser(a.handleBalance))
	e.GET("/user", a.withUser(a.handleUser))
	e.POST("/credit", a.withUser(a.handleCredit))
	e.POST("/debit", a.withUser(a.handleDebit))
	e.GET("/policy", a.withUser(a.handlePolicy))
	e.GET("/dev/boost", a.withUser(a.handleDevBoost))
	e.GET("/transactions", a.withUser(account.HandleGetTransactions(l)))

	// File upload (presigned URLs)
	e.POST("/files/upload-url", a.withUser(upload.HandleGetUploadURL(l, p, s3Client, nil, cfg.Credits.IngressPerGiB)))
	e.POST("/files/:uploadId/complete", a.withUser(upload.HandleCompleteUpload(s3Client, nil)))

	// Proxy upload endpoint (for CORS-safe upload in prod)
	e.POST("/files/:uploadId/upload", a.withUser(upload.HandleProxyUpload(s3Client)))

	// File operations (download, delete, list)
	downloadHandler := upload.NewDownloadHandler(l, p, s3Client, nil, cfg.Credits.EgressPerGiB)
	deletionHandler := upload.NewDeletionHandler(s3Client, nil, nil)

	e.GET("/files/:uploadID/download", a.withUser(downloadHandler.Download))
	e.DELETE("/files/:uploadID", a.withUser(deletionHandler.Delete))

	// File listing from S3
	e.GET("/files", a.withUser(upload.HandleListFiles(s3Client, cfg.Credits.IngressPerGiB, cfg.Credits.EgressPerGiB, cfg.Credits.StoragePerGiBMonth)))

	// Sharing endpoints
	var sh sharestore.Store
	if cfg.Server.Redis.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Redis.Host, cfg.Server.Redis.Port)
		st, err := sharestore.NewRedis(addr, cfg.Server.Redis.DB, cfg.Server.Redis.Password, "selfstack:share:")
		if err != nil {
			log.Fatalf("Failed to create Redis share store: %v", err)
		}
		sh = st
		log.Printf("Using Redis share store at %s (db=%d)", addr, cfg.Server.Redis.DB)
	} else {
		sh = sharestore.NewMemory()
		log.Printf("Using in-memory share store (not persistent!)")
	}
	e.POST("/files/:uploadId/share", a.withUser(share.HandleCreateShare(sh, s3Client)))
	e.GET("/share/:token", share.HandleRedeemShare(sh, l, p, s3Client, cfg.Credits.EgressPerGiB))

	// Start HTTP server
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	log.Printf("Starting PROD server on :%d (Phase 5 S3 storage)", port)
	log.Fatal(e.Start(":8080"))
}
