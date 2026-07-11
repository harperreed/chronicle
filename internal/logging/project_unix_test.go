// ABOUTME: Unix filesystem behavior tests for project log writing
// ABOUTME: Validates file modes and real write failures without test doubles
//go:build darwin || linux

package logging

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestWriteProjectLogCreatesFileWithRequestedMode(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "logs")
	oldUmask := syscall.Umask(0)
	t.Cleanup(func() {
		syscall.Umask(oldUmask)
	})

	entry := Entry{Timestamp: time.Date(2025, 11, 29, 14, 30, 0, 0, time.Local)}
	if err := WriteProjectLog(logDir, "markdown", entry); err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(logDir, "2025-11-29.log"))
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0644); got != want {
		t.Fatalf("log file mode = %04o, want %04o", got, want)
	}
}

func TestWriteProjectLogReturnsWriteError(t *testing.T) {
	if os.Getenv("CHRONICLE_LOG_WRITE_LIMIT_HELPER") == "1" {
		runWriteLimitHelper(t)
		return
	}

	logDir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWriteProjectLogReturnsWriteError$")
	cmd.Env = append(os.Environ(),
		"CHRONICLE_LOG_WRITE_LIMIT_HELPER=1",
		"CHRONICLE_LOG_WRITE_LIMIT_DIR="+logDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("write-limit subprocess failed: %v\n%s", err, output)
	}
}

func runWriteLimitHelper(t *testing.T) {
	signal.Ignore(syscall.SIGXFSZ)

	var originalLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &originalLimit); err != nil {
		t.Fatalf("Getrlimit failed: %v", err)
	}
	limit := originalLimit
	limit.Cur = 0
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatalf("Setrlimit failed: %v", err)
	}
	limitRestored := false
	t.Cleanup(func() {
		if !limitRestored {
			if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &originalLimit); err != nil {
				t.Errorf("failed to restore RLIMIT_FSIZE: %v", err)
			}
		}
	})

	logDir := os.Getenv("CHRONICLE_LOG_WRITE_LIMIT_DIR")
	entry := Entry{Timestamp: time.Date(2025, 11, 29, 14, 30, 0, 0, time.Local)}
	err := WriteProjectLog(logDir, "markdown", entry)
	if restoreErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &originalLimit); restoreErr != nil {
		t.Fatalf("failed to restore RLIMIT_FSIZE: %v", restoreErr)
	}
	limitRestored = true
	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("WriteProjectLog error = %v, want %v", err, syscall.EFBIG)
	}

	logFile := filepath.Join(logDir, "2025-11-29.log")
	info, statErr := os.Stat(logFile)
	if statErr != nil {
		t.Fatalf("log file was not opened and created before the write failed: %v", statErr)
	}
	if info.Size() != 0 {
		t.Fatalf("log file size = %d, want 0 after rejected write", info.Size())
	}
}
