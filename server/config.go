package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Auth         AuthConfig         `yaml:"auth"`
	CORS         CORSConfig         `yaml:"cors"`
	Features     FeaturesConfig     `yaml:"features"`
	Credits      CreditsConfig      `yaml:"credits"`
	Monetization MonetizationConfig `yaml:"monetization"`
	S3           S3Config           `yaml:"s3"`
}

type ServerConfig struct {
	Port  int         `yaml:"port"`
	Redis RedisConfig `yaml:"redis"`
}

type S3Config struct {
	Endpoint                 string `yaml:"endpoint"`
	Region                   string `yaml:"region"`
	Bucket                   string `yaml:"bucket"`
	AccessKey                string `yaml:"access_key"`
	SecretKey                string `yaml:"secret_key"`
	PresignedURLExpiryUpload int    `yaml:"presigned_url_expiry_upload"`   // seconds
	PresignedURLExpiryDL     int    `yaml:"presigned_url_expiry_download"` // seconds
}

type AuthConfig struct {
	Enabled          bool   `yaml:"enabled"`
	GoogleClientPath string `yaml:"google_client_path"`
	GoogleClientID   string `yaml:"google_client_id"`
}

type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"`
}

// FeaturesConfig holds feature flags that can be enabled/disabled.
type FeaturesConfig struct {
	EnableBoost bool `yaml:"enable_boost"`
}

// MonetizationConfig holds Web Monetization settings.
type MonetizationConfig struct {
	Enabled           bool   `yaml:"enabled"`
	PaymentPointer    string `yaml:"payment_pointer"`
	MinTopupUnits     int64  `yaml:"min_topup_units"`     // min minor units accepted
	MaxTopupUnits     int64  `yaml:"max_topup_units"`     // max minor units accepted
	IncomingAuthToken string `yaml:"incoming_auth_token"` // optional bearer token to fetch incoming payments
}

// CreditsConfig holds credit system settings.
type CreditsConfig struct {
	Scale              int   `yaml:"scale"`                    // decimal scale for credits (e.g., 4 = 4 decimal places)
	UnitsPerCredit     int64 `yaml:"units_per_credit"`         // currency minor units required for 1 credit
	IngressPerGiB      int64 `yaml:"ingress_per_gib"`          // credit cost per GiB uploaded (internal raw units, auto-converted from float config)
	EgressPerGiB       int64 `yaml:"egress_per_gib"`           // credit cost per GiB downloaded (internal raw units, auto-converted from float config)
	StoragePerGiBMonth int64 `yaml:"storage_per_gib_month"`    // credit cost per GiB stored per month (internal raw units, auto-converted from float config)
	ChargeFrequencyMin int64 `yaml:"charge_frequency_minutes"` // how often to charge storage, in minutes
}

// RedisConfig holds Redis connection details.
type RedisConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	DB        int    `yaml:"db"`
	Password  string `yaml:"password"`
	KeyPrefix string `yaml:"key_prefix"`
}

var cfg *Config

// creditUnitFromScale returns 10^scale as int64
func creditUnitFromScale(scale int) int64 {
	if scale <= 0 {
		return 1
	}
	var u int64 = 1
	for i := 0; i < scale; i++ {
		u *= 10
	}
	return u
}

// StorageChargePerGiBInterval returns the charge to apply per GiB for the configured interval.
// Calculation: (storage_per_gib_month / minutes_in_month) * charge_frequency_minutes, rounded down.
func (c CreditsConfig) StorageChargePerGiBInterval() int64 {
	const minutesInMonth = int64(30 * 24 * 60) // 43,200
	if c.ChargeFrequencyMin <= 0 {
		return 0
	}
	if c.StoragePerGiBMonth <= 0 {
		return 0
	}
	return (c.StoragePerGiBMonth * c.ChargeFrequencyMin) / minutesInMonth
}

// LoadConfig loads configuration from a YAML file (JSON compatible for legacy configs)
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// First, unmarshal with a temporary struct that accepts float values for pricing
	var rawData struct {
		Server       interface{}            `yaml:"server"`
		Auth         interface{}            `yaml:"auth"`
		CORS         interface{}            `yaml:"cors"`
		Features     interface{}            `yaml:"features"`
		Credits      map[string]interface{} `yaml:"credits"`
		Monetization interface{}            `yaml:"monetization"`
		S3           interface{}            `yaml:"s3"`
	}
	if err := yaml.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Now parse the entire config normally
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Convert float credit prices from admin-friendly config to raw internal units
	if rawData.Credits != nil {
		creditUnit := creditUnitFromScale(config.Credits.Scale)

		// Convert ingress_per_gib from float to raw units
		if val, ok := rawData.Credits["ingress_per_gib"].(float64); ok && val > 0 {
			config.Credits.IngressPerGiB = int64(val * float64(creditUnit))
		}

		// Convert egress_per_gib from float to raw units
		if val, ok := rawData.Credits["egress_per_gib"].(float64); ok && val > 0 {
			config.Credits.EgressPerGiB = int64(val * float64(creditUnit))
		}

		// Convert storage_per_gib_month from float to raw units
		if val, ok := rawData.Credits["storage_per_gib_month"].(float64); ok && val > 0 {
			config.Credits.StoragePerGiBMonth = int64(val * float64(creditUnit))
		}
	}

	// Validate S3 configuration (mandatory)
	if config.S3.Endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required (s3.endpoint)")
	}
	if config.S3.Region == "" {
		return nil, fmt.Errorf("S3 region is required (s3.region)")
	}
	if config.S3.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required (s3.bucket)")
	}
	// Fall back to environment variables if static keys are not provided in config
	if config.S3.AccessKey == "" {
		config.S3.AccessKey = os.Getenv("S3_ACCESS_KEY")
	}
	if config.S3.SecretKey == "" {
		config.S3.SecretKey = os.Getenv("S3_SECRET_KEY")
	}
	if config.S3.AccessKey == "" {
		return nil, fmt.Errorf("S3 access key is required (s3.access_key or S3_ACCESS_KEY env)")
	}
	if config.S3.SecretKey == "" {
		return nil, fmt.Errorf("S3 secret key is required (s3.secret_key or S3_SECRET_KEY env)")
	}

	// Set S3 expiry defaults if not specified
	if config.S3.PresignedURLExpiryUpload == 0 {
		config.S3.PresignedURLExpiryUpload = 3600 // 1 hour
	}
	if config.S3.PresignedURLExpiryDL == 0 {
		config.S3.PresignedURLExpiryDL = 300 // 5 minutes
	}

	// Set credit defaults if not specified (after float conversion above)
	if config.Credits.Scale == 0 {
		config.Credits.Scale = 4 // 4 decimal places
	}
	creditUnit := creditUnitFromScale(config.Credits.Scale)
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
		config.Credits.ChargeFrequencyMin = 60 // default: hourly storage charging
	}

	// Set sane defaults for Redis
	if config.Server.Redis.Port == 0 {
		config.Server.Redis.Port = 6379
	}
	if config.Server.Redis.Host == "" {
		config.Server.Redis.Host = "127.0.0.1"
	}
	if config.Server.Redis.KeyPrefix == "" {
		config.Server.Redis.KeyPrefix = "microvault:credits:"
	}

	// Default price per credit (minor currency units per credit)
	if config.Credits.UnitsPerCredit == 0 {
		// Default: 10 minor units per 1 credit
		config.Credits.UnitsPerCredit = 10
	}

	// If Google client path is specified, load client ID from there
	if config.Auth.GoogleClientPath != "" {
		clientID, err := extractClientIDFromGCP(config.Auth.GoogleClientPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load Google client: %w", err)
		}
		config.Auth.GoogleClientID = clientID
	}

	cfg = &config
	return &config, nil
}

// extractClientIDFromGCP extracts client_id from GCP credentials JSON
func extractClientIDFromGCP(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var gcpCreds struct {
		Web struct {
			ClientID string `json:"client_id"`
		} `json:"web"`
	}

	if err := json.Unmarshal(data, &gcpCreds); err != nil {
		return "", err
	}

	if gcpCreds.Web.ClientID == "" {
		return "", fmt.Errorf("client_id not found in GCP credentials")
	}

	return gcpCreds.Web.ClientID, nil
}

// GetConfig returns the loaded configuration
func GetConfig() *Config {
	return cfg
}
