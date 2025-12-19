package main

import (
	"os"
	"testing"
)

// init is called at package initialization - runs before any tests
func init() {
	DevMode = true
}

// TestMain is called before any tests run
func TestMain(m *testing.M) {
	DevMode = true
	exitCode := m.Run()
	os.Exit(exitCode)
}
