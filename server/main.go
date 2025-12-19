package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Global variable to control dev mode (set via CLI flag)
var DevMode bool

// getDefaultConfigPath returns a local config file, preferring YAML
func getDefaultConfigPath() string {
	if _, err := os.Stat(".localconfig.yaml"); err == nil {
		return ".localconfig.yaml"
	}
	if _, err := os.Stat(".localconfig.yml"); err == nil {
		return ".localconfig.yml"
	}
	if _, err := os.Stat(".localconfig.json"); err == nil {
		return ".localconfig.json"
	}
	return "config.yaml"
}

func main() {
	// Define command-line flags
	configPath := flag.String("config", "", "path to configuration file")
	devModeFlag := flag.Bool("dev", false, "enable development mode (dev-login auth, simplified setup)")
	help := flag.Bool("help", false, "show help message")
	flag.BoolVar(help, "h", false, "show help message")

	flag.Usage = func() {
		helpText := "MicroVault - File Storage Service\n\n" +
			"Usage:\n" +
			"  microvault [options]                    Start server\n" +
			"  microvault <command> [args] [options]   Run admin command\n\n" +
			"Server Options:\n" +
			"  -config string\n" +
			"        path to configuration file\n" +
			"        - for dev mode: defaults to .localconfig.yaml/.yml if present (falls back to .json)\n" +
			"        - for prod mode: required or use CONFIG_PATH env var\n" +
			"  -dev\n" +
			"        enable development mode (X-User-ID auth, in-memory ledger, simple setup)\n" +
			"  -help, -h\n" +
			"        show this help message\n\n" +
			"Admin Commands:\n" +
			"  list                          List all users with balances and storage info\n" +
			"  delete <userID>               Delete user and all their data from system\n" +
			"  drain <userID>                Set user balance to zero\n" +
			"  boost <userID> <amount>       Add credits to user account (in credit units)\n\n" +
			"Environment Variables (Production):\n" +
			"  CONFIG_PATH\n" +
			"        path to config.yaml (overridden by -config flag)\n\n" +
			"Server Examples:\n" +
			"  # Development mode (auto-detects .localconfig.yaml/.yml)\n" +
			"  microvault -dev\n\n" +
			"  # Development mode with explicit config\n" +
			"  microvault -dev -config my-config.yaml\n\n" +
			"  # Production mode\n" +
			"  CONFIG_PATH=/etc/microvault/prod.yaml microvault\n\n" +
			"  # Production with explicit config\n" +
			"  microvault -config /etc/microvault/prod.yaml\n\n" +
			"Admin Examples:\n" +
			"  # List all users (flags must come BEFORE command)\n" +
			"  microvault -config /etc/microvault/config.yaml list\n\n" +
			"  # Delete a user\n" +
			"  microvault -config /etc/microvault/config.yaml delete user@example.com\n\n" +
			"  # Drain user credits\n" +
			"  microvault -config /etc/microvault/config.yaml drain user@example.com\n\n" +
			"  # Boost user with 100,000 credit units (10.0000 credits with scale=4)\n" +
			"  microvault -config /etc/microvault/config.yaml boost user@example.com 100000\n"

		fmt.Fprint(os.Stderr, helpText)
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Check if running as CLI command
	args := flag.Args()
	if len(args) > 0 {
		command := args[0]
		commandArgs := args[1:]

		// Determine config path for CLI
		configFile := *configPath
		if configFile == "" {
			configFile = os.Getenv("CONFIG_PATH")
			if configFile == "" {
				// Prefer YAML, but allow legacy JSON
				if _, err := os.Stat("config.yaml"); err == nil {
					configFile = "config.yaml"
				} else if _, err := os.Stat("config.yml"); err == nil {
					configFile = "config.yml"
				} else {
					configFile = "config.json"
				}
			}
		}

		runCLI(command, commandArgs, configFile)
		return
	}

	// Otherwise, start server
	// Set global dev mode flag
	DevMode = *devModeFlag

	// Determine config path
	configFile := *configPath
	if configFile == "" {
		if DevMode {
			configFile = getDefaultConfigPath()
		} else {
			configFile = os.Getenv("CONFIG_PATH")
			if configFile == "" {
				if _, err := os.Stat("/etc/microvault/config.yaml"); err == nil {
					configFile = "/etc/microvault/config.yaml"
				} else if _, err := os.Stat("/etc/microvault/config.yml"); err == nil {
					configFile = "/etc/microvault/config.yml"
				} else {
					configFile = "/etc/microvault/config.json"
				}
			}
		}
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded from: %s", configFile)
	log.Printf("Operating mode: %s", modeString())

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	if DevMode {
		initDevMode(e, cfg)
	} else {
		initProdMode(e, cfg)
	}
}

func modeString() string {
	if DevMode {
		return "DEVELOPMENT (X-User-ID auth)"
	}
	return "PRODUCTION (Google OAuth)"
}

// Assert interface satisfaction at compile time.
// Errors exposed for future wiring/testing.
var (
	errUnauthorized = errors.New("missing user")
)
