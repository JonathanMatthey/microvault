package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/bosbaber/hackweek/microvault/internal/account"
	"github.com/bosbaber/hackweek/microvault/internal/activity"
	"github.com/bosbaber/hackweek/microvault/internal/ledger/memory"
	"github.com/bosbaber/hackweek/microvault/internal/policy"
	"github.com/bosbaber/hackweek/microvault/internal/share"
	sharestore "github.com/bosbaber/hackweek/microvault/internal/share/store"
	"github.com/bosbaber/hackweek/microvault/internal/storage"
	"github.com/bosbaber/hackweek/microvault/internal/upload"
)

func initDevMode(e *echo.Echo, cfg *Config) {
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

	// Enable CORS for frontend
	e.Use(middleware.CORSWithConfig(buildCORSConfig(cfg)))

	// Initialize core services
	l := memory.New()
	// Create policy engine with minimum cost of 1 credit unit
	p := policy.New(1)
	act := activity.New()

	a := &app{
		ledger:   l,
		policy:   p,
		activity: act,
	}

	// Start periodic storage charging in dev (helps validate billing logic)
	chargePerInterval := cfg.Credits.StorageChargePerGiBInterval()
	charger := activity.NewStorageCharger(l, s3Client, cfg.Credits.ChargeFrequencyMin, chargePerInterval)
	charger.Start()

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok", "mode": "dev"})
	})

	// Monetization config (dev stub)
	e.GET("/monetization/config", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled":          false,
			"payment_pointer":  "",
			"asset_code":       "USD",
			"asset_scale":      2,
			"units_per_credit": cfg.Credits.UnitsPerCredit,
			"min_topup_units":  int64(0),
			"max_topup_units":  int64(0),
		})
	})

	// Monetization endpoints (no-op stubs)
	e.POST("/monetization/verify", a.withUser(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"credits": a.ledger.Balance(c.Get("userID").(string)),
		})
	}))

	e.POST("/monetization/receipt", a.withUser(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"credits": a.ledger.Balance(c.Get("userID").(string)),
		})
	}))

	// Account management (dev X-User-ID auth)
	e.POST("/login", a.handleLogin)
	e.GET("/balance", a.withUser(a.handleBalance))
	e.GET("/user", a.withUser(a.handleUser))
	e.POST("/credit", a.withUser(a.handleCredit))
	e.POST("/debit", a.withUser(a.handleDebit))
	e.GET("/policy", a.withUser(a.handlePolicy))
	e.GET("/dev/boost", a.withUser(a.handleDevBoost))
	e.GET("/transactions", a.withUser(account.HandleGetTransactions(l)))

	// File upload (presigned URLs)
	e.POST("/files/upload-url", a.withUser(upload.HandleGetUploadURL(l, p, s3Client, nil, 15000))) // 1.5 credits per GiB
	e.POST("/files/:uploadId/complete", a.withUser(upload.HandleCompleteUpload(s3Client, nil)))

	// Proxy upload endpoint (for CORS-safe upload in dev mode)
	e.POST("/files/:uploadId/upload", a.withUser(upload.HandleProxyUpload(s3Client)))

	// File operations (download, delete, list)
	downloadHandler := upload.NewDownloadHandler(l, p, s3Client, nil, 30000) // 3.0 credits per GiB
	deletionHandler := upload.NewDeletionHandler(s3Client, nil, nil)

	e.GET("/files/:uploadID/download", a.withUser(downloadHandler.Download))
	e.DELETE("/files/:uploadID", a.withUser(deletionHandler.Delete))

	// File listing from S3
	e.GET("/files", a.withUser(upload.HandleListFiles(s3Client, cfg.Credits.IngressPerGiB, cfg.Credits.EgressPerGiB, cfg.Credits.StoragePerGiBMonth)))

	// Sharing endpoints (memory store in dev)
	sh := sharestore.NewMemory()
	e.POST("/files/:uploadId/share", a.withUser(share.HandleCreateShare(sh, s3Client)))
	e.GET("/share/:token", share.HandleRedeemShare(sh, l, p, s3Client, 30000))

	// Start HTTP server
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	log.Printf("Starting DEV server on :%d (X-User-ID auth, S3 storage)", port)
	log.Fatal(e.Start(":8080"))
}
