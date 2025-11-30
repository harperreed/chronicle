//go:build sqlite_fts5

// ABOUTME: Unit tests for the add command
// ABOUTME: Tests message handling and tag flag variations
package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// executeAndCaptureOutput runs the command and returns output and error.
func executeAndCaptureOutput(args []string) (string, error) {
	tags = []string{} // Reset tags

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String(), err
}

// assertSuccessOutput checks that output contains expected success messages.
func assertSuccessOutput(t *testing.T, output string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(output, "Entry created") {
		t.Errorf("expected output to contain 'Entry created', got: %s", output)
	}
	if !strings.Contains(output, "ID:") {
		t.Errorf("expected output to contain ID, got: %s", output)
	}
}

func TestAddCommand(t *testing.T) {
	t.Run("accepts message with long-form tags", func(t *testing.T) {
		output, err := executeAndCaptureOutput([]string{"add", "test message", "--tag", "work", "--tag", "important"})
		assertSuccessOutput(t, output, err)
	})

	t.Run("accepts message with short-form tags", func(t *testing.T) {
		output, err := executeAndCaptureOutput([]string{"add", "short form test", "-t", "personal"})
		assertSuccessOutput(t, output, err)
	})

	t.Run("accepts multiple tags", func(t *testing.T) {
		output, err := executeAndCaptureOutput([]string{"add", "multi tag message", "-t", "tag1", "-t", "tag2", "--tag", "tag3"})
		assertSuccessOutput(t, output, err)
	})

	t.Run("rejects no arguments", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		rootCmd.SetArgs([]string{"add"})
		err := rootCmd.Execute()

		if err == nil {
			t.Fatal("expected error when no arguments provided, got nil")
		}

		if !strings.Contains(err.Error(), "1 arg(s)") && !strings.Contains(err.Error(), "requires") {
			t.Errorf("expected error message about required args, got: %v", err)
		}
	})

	t.Run("rejects too many arguments", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		rootCmd.SetArgs([]string{"add", "message one", "message two"})
		err := rootCmd.Execute()

		if err == nil {
			t.Fatal("expected error when too many arguments provided, got nil")
		}

		if !strings.Contains(err.Error(), "1 arg(s)") && !strings.Contains(err.Error(), "accepts") {
			t.Errorf("expected error message about exact args, got: %v", err)
		}
	})
}
