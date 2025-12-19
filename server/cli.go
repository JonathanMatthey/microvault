package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	redisledger "github.com/bosbaber/hackweek/selfstack/internal/ledger/redis"
	"github.com/bosbaber/hackweek/selfstack/internal/storage"
	"gopkg.in/yaml.v3"
)

// loadConfigForCLI loads config without requiring Google auth setup
func loadConfigForCLI(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults (skip Google auth validation)
	if config.S3.PresignedURLExpiryUpload == 0 {
		config.S3.PresignedURLExpiryUpload = 3600
	}
	if config.S3.PresignedURLExpiryDL == 0 {
		config.S3.PresignedURLExpiryDL = 300
	}
	if config.Credits.Scale == 0 {
		config.Credits.Scale = 4
	}
	creditUnit := func(scale int) int64 {
		if scale <= 0 {
			return 1
		}
		var u int64 = 1
		for i := 0; i < scale; i++ {
			u *= 10
		}
		return u
	}(config.Credits.Scale)

	if config.Credits.IngressPerGiB == 0 {
		config.Credits.IngressPerGiB = creditUnit / 10 // 0.1 credits
	}
	if config.Credits.EgressPerGiB == 0 {
		config.Credits.EgressPerGiB = creditUnit * 2 // 2.0 credits
	}
	if config.Credits.StoragePerGiBMonth == 0 {
		config.Credits.StoragePerGiBMonth = creditUnit * 10 // 10.0 credits per GiB-month
	}
	if config.Credits.ChargeFrequencyMin == 0 {
		config.Credits.ChargeFrequencyMin = 60
	}
	if config.Server.Redis.Port == 0 {
		config.Server.Redis.Port = 6379
	}
	if config.Server.Redis.Host == "" {
		config.Server.Redis.Host = "127.0.0.1"
	}
	if config.Server.Redis.KeyPrefix == "" {
		config.Server.Redis.KeyPrefix = "selfstack:credits:"
	}

	return &config, nil
}

// runCLI executes CLI commands for admin operations
func runCLI(command string, args []string, configPath string) {
	cfg, err := loadConfigForCLI(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize ledger (prefer Redis if available)
	var l interface {
		Balance(userID string) int64
		Credit(userID string, amount int64) error
		Debit(userID string, amount int64) error
		ListAll() (map[string]int64, error)
	}

	if cfg.Server.Redis.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Redis.Host, cfg.Server.Redis.Port)
		redisLedger, err := redisledger.New(addr, cfg.Server.Redis.DB, cfg.Server.Redis.Password, cfg.Server.Redis.KeyPrefix)
		if err != nil {
			log.Fatalf("Failed to connect to Redis: %v", err)
		}
		l = redisLedger
		log.Printf("Connected to Redis ledger at %s", addr)
	} else {
		log.Fatalf("CLI commands require Redis ledger (in-memory ledger not supported for admin operations)")
	}

	// Initialize S3 client
	s3Client, err := storage.NewS3Client(storage.S3Config{
		Endpoint:  cfg.S3.Endpoint,
		Region:    cfg.S3.Region,
		Bucket:    cfg.S3.Bucket,
		AccessKey: cfg.S3.AccessKey,
		SecretKey: cfg.S3.SecretKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	switch command {
	case "list":
		listUsers(l, s3Client, cfg)
	case "delete":
		if len(args) < 1 {
			log.Fatalf("Usage: selfstack delete <userID>")
		}
		deleteUser(args[0], l, s3Client, cfg)
	case "drain":
		if len(args) < 1 {
			log.Fatalf("Usage: selfstack drain <userID>")
		}
		drainUser(args[0], l, cfg)
	case "boost":
		if len(args) < 2 {
			log.Fatalf("Usage: selfstack boost <userID> <amount>")
		}
		var amount int64
		if _, err := fmt.Sscanf(args[1], "%d", &amount); err != nil {
			log.Fatalf("Invalid amount: %v", err)
		}
		boostUser(args[0], amount, l, cfg)
	case "dropall":
		dropAllUsers(l, s3Client, cfg)
	default:
		log.Fatalf("Unknown command: %s", command)
	}

}

// dropAllUsers removes all users and their balances from the ledger and S3
func dropAllUsers(l interface {
	ListAll() (map[string]int64, error)
	Debit(userID string, amount int64) error
}, s3Client *storage.S3Client, cfg *Config) {
	balances, err := l.ListAll()
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	for userID := range balances {
		// Remove all files for user
		userHash := storage.UserHashFromID(userID)
		files, err := s3Client.ListUserFiles(context.Background(), userHash)
		if err == nil {
			for _, file := range files {
				_ = s3Client.DeleteObject(context.Background(), userHash, file.UploadID)
			}
		}
		// Remove balance by debiting full amount
		_ = l.Debit(userID, balances[userID])
		fmt.Printf("Deleted user: %s\n", userID)
	}
	fmt.Println("All users and their data have been dropped.")
}

func listUsers(l interface {
	Balance(userID string) int64
	ListAll() (map[string]int64, error)
}, s3Client *storage.S3Client, cfg *Config) {
	balances, err := l.ListAll()
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}

	// Get storage info for each user
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USER ID\tBALANCE\tFILES\tTOTAL SIZE")
	fmt.Fprintln(w, "-------\t-------\t-----\t----------")

	for userID, balance := range balances {
		userHash := storage.UserHashFromID(userID)
		files, err := s3Client.ListUserFiles(context.Background(), userHash)

		fileCount := 0
		var totalSize int64
		if err == nil {
			fileCount = len(files)
			for _, file := range files {
				totalSize += file.Size
			}
		}

		// Display balance as credits with decimal places
		unit := creditUnitFromScale(cfg.Credits.Scale)
		displayBalance := float64(balance) / float64(unit)
		fmt.Fprintf(w, "%s\t%.4f\t%d\t%s\n",
			userID,
			displayBalance,
			fileCount,
			formatBytes(totalSize))
	}
	w.Flush()
}

func deleteUser(userID string, l interface {
	Balance(userID string) int64
	Credit(userID string, amount int64) error
	Debit(userID string, amount int64) error
}, s3Client *storage.S3Client, cfg *Config) {
	// Get current balance
	balance := l.Balance(userID)

	// Delete all files from S3
	userHash := storage.UserHashFromID(userID)
	files, err := s3Client.ListUserFiles(context.Background(), userHash)
	if err != nil {
		log.Printf("Warning: Failed to list files for user %s: %v", userID, err)
	}

	deletedCount := 0
	for _, file := range files {
		if err := s3Client.DeleteObject(context.Background(), userHash, file.UploadID); err != nil {
			log.Printf("Warning: Failed to delete %s: %v", file.UploadID, err)
		} else {
			deletedCount++
		}
	}

	// Zero out balance
	if balance > 0 {
		if err := l.Debit(userID, balance); err != nil {
			log.Printf("Warning: Failed to zero balance: %v", err)
		}
	} else if balance < 0 {
		// If balance was negative, credit to bring to zero
		if err := l.Credit(userID, -balance); err != nil {
			log.Printf("Warning: Failed to zero balance: %v", err)
		}
	}

	log.Printf("Deleted user %s: removed %d files, zeroed balance", userID, deletedCount)
}

func drainUser(userID string, l interface {
	Balance(userID string) int64
	Debit(userID string, amount int64) error
}, cfg *Config) {
	balance := l.Balance(userID)
	if balance <= 0 {
		unit := creditUnitFromScale(cfg.Credits.Scale)
		log.Printf("User %s already has zero or negative balance: %.4f",
			userID,
			float64(balance)/float64(unit))
		return
	}

	if err := l.Debit(userID, balance); err != nil {
		log.Fatalf("Failed to drain user: %v", err)
	}

	unit := creditUnitFromScale(cfg.Credits.Scale)
	log.Printf("Drained user %s: removed %.4f credits",
		userID,
		float64(balance)/float64(unit))
}

func boostUser(userID string, amount int64, l interface {
	Balance(userID string) int64
	Credit(userID string, amount int64) error
}, cfg *Config) {
	oldBalance := l.Balance(userID)

	if err := l.Credit(userID, amount); err != nil {
		log.Fatalf("Failed to boost user: %v", err)
	}

	newBalance := l.Balance(userID)
	unit := creditUnitFromScale(cfg.Credits.Scale)
	log.Printf("Boosted user %s: %.4f -> %.4f credits (+%.4f)",
		userID,
		float64(oldBalance)/float64(unit),
		float64(newBalance)/float64(unit),
		float64(amount)/float64(unit))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
