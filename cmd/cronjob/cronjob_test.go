package main

import (
	"log/slog"
	"os"
	"testing"
)

func TestDebug(t *testing.T) {
	// Test case 1: DEBUG environment variable is not set or empty
	os.Unsetenv("DEBUG") // Ensure the variable is not set
	expectedLevel := slog.LevelInfo
	actualLevel := debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG is not set, but got %v", expectedLevel, actualLevel)
	}

	// Test case 2: DEBUG environment variable is set to "1"
	os.Setenv("DEBUG", "1")
	expectedLevel = slog.LevelDebug
	actualLevel = debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG=1, but got %v", expectedLevel, actualLevel)
	}

	// Test case 3: DEBUG environment variable is set to something else (e.g., "0")
	os.Setenv("DEBUG", "0")
	expectedLevel = slog.LevelInfo // Should default to Info
	actualLevel = debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG is not '1', but got %v", expectedLevel, actualLevel)
	}

	// Clean up the environment variable after the test
	os.Unsetenv("DEBUG")
}
