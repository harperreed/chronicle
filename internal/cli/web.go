// ABOUTME: Web command for serving Chronicle's local browser interface.
// ABOUTME: Opens configured storage and shuts the HTTP server down gracefully.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/harper/chronicle/internal/config"
	chronicleweb "github.com/harper/chronicle/internal/web"
	"github.com/spf13/cobra"
)

var (
	webAddress     string
	webOpen        bool
	webAllowRemote bool
	webReadOnly    bool
)

var webCmd = &cobra.Command{
	Use:     "web",
	Aliases: []string{"serve"},
	Short:   "Open Chronicle's web interface",
	Long:    "Serve a local, private web interface for browsing, searching, creating, editing, and deleting Chronicle entries.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		app, closeStorage, err := newWebApp(cfg, webReadOnly, chronicleweb.Options{AllowRemote: webAllowRemote})
		if err != nil {
			return err
		}
		defer func() { _ = closeStorage() }()

		listener, err := net.Listen("tcp", webAddress)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", webAddress, err)
		}
		defer func() { _ = listener.Close() }()

		server := &http.Server{
			Handler:           app.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		// Shutdown waits for open requests, and a stream never ends on its own.
		server.RegisterOnShutdown(app.StopStreams)
		url := browserURL(listener.Addr())
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Chronicle web interface: %s\nPress Ctrl+C to stop.\n", url)
		if webOpen {
			go func() { _ = openBrowser(url) }()
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		// After the first signal, restore the default signal behavior, so that a second Ctrl+C
		// ends the process at once instead of waiting out a slow shutdown.
		context.AfterFunc(ctx, stop)
		return serveUntilStopped(ctx, server, listener)
	},
}

// webShutdownTimeout is how long a stopping web server waits for open requests to finish.
const webShutdownTimeout = 5 * time.Second

// serveUntilStopped serves on listener until ctx ends, then shuts the server down. It returns
// once the shutdown has finished, so requests that were running when ctx ended get to complete,
// within webShutdownTimeout. The caller can then close the storage they use.
func serveUntilStopped(ctx context.Context, server *http.Server, listener net.Listener) error {
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		// ctx has ended, so the shutdown needs a context of its own to keep its time limit.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), webShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve web interface: %w", err)
	}
	// Serve returns as soon as Shutdown starts, before the open requests have finished.
	<-shutdownDone
	return nil
}

// newWebApp opens the configured storage and builds the web server over it. With readOnly set,
// the storage opens for reading only and the server has no way to write. The function it returns
// closes the storage.
func newWebApp(cfg *config.Config, readOnly bool, options chronicleweb.Options) (*chronicleweb.Server, func() error, error) {
	if readOnly {
		reader, err := cfg.OpenReader()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open storage: %w", err)
		}
		app, err := chronicleweb.NewReadOnly(reader, options)
		if err != nil {
			_ = reader.Close()
			return nil, nil, err
		}
		return app, reader.Close, nil
	}
	store, err := cfg.OpenStorage()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open storage: %w", err)
	}
	app, err := chronicleweb.New(store, options)
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	return app, store.Close, nil
}

func browserURL(address net.Addr) string {
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return "http://" + address.String()
	}
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url) // #nosec G204 -- URL is generated from the bound listener, not user shell input.
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) // #nosec G204 -- see above.
	default:
		command = exec.Command("xdg-open", url) // #nosec G204 -- see above.
	}
	return command.Start()
}

func init() {
	webCmd.Flags().StringVar(&webAddress, "address", "127.0.0.1:1066", "Address to listen on (use :0 for a random port)")
	webCmd.Flags().BoolVar(&webOpen, "open", true, "Open the interface in your default browser")
	webCmd.Flags().BoolVar(&webAllowRemote, "allow-remote", false, "Allow non-loopback Host headers (requires your own authentication proxy)")
	webCmd.Flags().BoolVar(&webReadOnly, "read-only", false, "Serve a live view that can't create, edit or delete entries, and open storage read-only")
	rootCmd.AddCommand(webCmd)
}
