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

func TestAddCommand(t *testing.T) {
	t.Run("accepts message with long-form tags", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Set args and execute through root command
		rootCmd.SetArgs([]string{"add", "test message", "--tag", "work", "--tag", "important"})
		err := rootCmd.Execute()

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !strings.Contains(output, "test message") {
			t.Errorf("expected output to contain 'test message', got: %s", output)
		}
		if !strings.Contains(output, "work") || !strings.Contains(output, "important") {
			t.Errorf("expected output to contain tags, got: %s", output)
		}
	})

	t.Run("accepts message with short-form tags", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		rootCmd.SetArgs([]string{"add", "short form test", "-t", "personal"})
		err := rootCmd.Execute()

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !strings.Contains(output, "short form test") {
			t.Errorf("expected output to contain 'short form test', got: %s", output)
		}
		if !strings.Contains(output, "personal") {
			t.Errorf("expected output to contain 'personal' tag, got: %s", output)
		}
	})

	t.Run("accepts multiple tags", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		rootCmd.SetArgs([]string{"add", "multi tag message", "-t", "tag1", "-t", "tag2", "--tag", "tag3"})
		err := rootCmd.Execute()

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !strings.Contains(output, "multi tag message") {
			t.Errorf("expected output to contain message, got: %s", output)
		}
		if !strings.Contains(output, "tag1") || !strings.Contains(output, "tag2") || !strings.Contains(output, "tag3") {
			t.Errorf("expected output to contain all tags, got: %s", output)
		}
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
