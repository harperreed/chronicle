// ABOUTME: Tests for how the web command opens storage, serves and shuts down.
// ABOUTME: They run real stores in temp folders and a real HTTP server on a loopback port.

package cli

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/config"
	chronicleweb "github.com/harper/chronicle/internal/web"
)

func TestWebFlagDefaults(t *testing.T) {
	address := webCmd.Flags().Lookup("address")
	if address == nil {
		t.Fatal("web has no --address flag")
	}
	if address.DefValue != "127.0.0.1:1066" {
		t.Errorf("--address defaults to %q, want 127.0.0.1:1066", address.DefValue)
	}

	readOnly := webCmd.Flags().Lookup("read-only")
	if readOnly == nil {
		t.Fatal("web has no --read-only flag")
	}
	if readOnly.DefValue != "false" {
		t.Errorf("--read-only defaults to %q, want false: the full page is the default", readOnly.DefValue)
	}
}

// A read-only web app must not create the data folder, so a viewer pointed at a log that does not
// exist yet leaves no trace. It must also refuse to change entries, since a store that opened
// read-only has nothing to write with.
func TestNewWebAppReadOnlyMarkdownLeavesAbsentDataFolderAbsent(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "chronicle")
	cfg := &config.Config{Backend: "markdown", DataDir: dataDir}

	app, closeStorage, err := newWebApp(cfg, true, chronicleweb.Options{})
	if err != nil {
		t.Fatalf("newWebApp(read-only) over an absent markdown folder = %v, want it to open", err)
	}
	defer func() {
		if err := closeStorage(); err != nil {
			t.Errorf("closing read-only storage = %v", err)
		}
	}()

	handler := app.Handler()
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1066/api/entries", nil))
	if list.Code != http.StatusOK {
		t.Errorf("GET /api/entries = %d, want 200 with an empty list", list.Code)
	}
	create := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:1066/api/entries", strings.NewReader(`{"message":"from a viewer"}`))
	create.Header.Set("Content-Type", "application/json")
	refused := httptest.NewRecorder()
	handler.ServeHTTP(refused, create)
	if refused.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/entries = %d, want 405", refused.Code)
	}

	if _, err := os.Stat(dataDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the data folder %s exists after a read-only open and two requests (stat error = %v), want it absent", dataDir, err)
	}
}

// With no database, a read-only web app must fail with a clear message and create nothing, whether
// the data folder exists or not. Opening for writing would create the database.
func TestNewWebAppReadOnlySQLiteWithoutDatabaseFailsAndCreatesNothing(t *testing.T) {
	t.Run("data folder exists", func(t *testing.T) {
		dataDir := t.TempDir()
		requireNoDatabaseError(t, dataDir)

		entries, err := os.ReadDir(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("%s holds %d files after the failed open, want none: the first is %s", dataDir, len(entries), entries[0].Name())
		}
	})

	t.Run("data folder absent", func(t *testing.T) {
		dataDir := filepath.Join(t.TempDir(), "chronicle")
		requireNoDatabaseError(t, dataDir)

		if _, err := os.Stat(dataDir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the data folder %s exists after the failed open (stat error = %v), want it absent", dataDir, err)
		}
	})
}

// requireNoDatabaseError fails the test unless a read-only newWebApp over the SQLite data folder
// dataDir, which holds no database, returns the error that says so.
func requireNoDatabaseError(t *testing.T, dataDir string) {
	t.Helper()
	cfg := &config.Config{Backend: "sqlite", DataDir: dataDir}

	_, closeStorage, err := newWebApp(cfg, true, chronicleweb.Options{})
	if err == nil {
		_ = closeStorage()
		t.Fatal("newWebApp(read-only) without a database opened, want an error")
	}
	if !strings.Contains(err.Error(), "no chronicle database at") {
		t.Errorf("error = %q, want it to say \"no chronicle database at\"", err)
	}
}

func TestServeUntilStoppedWaitsForRunningRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release // Still running when the server is told to stop.
			_, _ = io.WriteString(w, "finished")
		}),
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	served := make(chan error, 1)
	go func() { served <- serveUntilStopped(ctx, server, listener) }()

	client := &http.Client{Timeout: 10 * time.Second}
	bodies := make(chan string, 1)
	go func() {
		response, err := client.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			bodies <- "request failed: " + err.Error()
			return
		}
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		bodies <- string(body)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the request never reached the handler")
	}

	stop()
	select {
	case err := <-served:
		t.Fatalf("serveUntilStopped returned (%v) while a request was still running", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serveUntilStopped = %v, want nil after a clean stop", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilStopped did not return after the request finished")
	}
	if got := <-bodies; got != "finished" {
		t.Fatalf("the request that was running when the server stopped got %q, want its response", got)
	}
}
