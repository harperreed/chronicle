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
		store, err := cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer func() { _ = store.Close() }()

		app, err := chronicleweb.New(store, chronicleweb.Options{AllowRemote: webAllowRemote})
		if err != nil {
			return err
		}
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
		url := browserURL(listener.Addr())
		fmt.Fprintf(cmd.OutOrStdout(), "Chronicle web interface: %s\nPress Ctrl+C to stop.\n", url)
		if webOpen {
			go func() { _ = openBrowser(url) }()
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve web interface: %w", err)
		}
		return nil
	},
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
	webCmd.Flags().StringVar(&webAddress, "address", "127.0.0.1:8080", "Address to listen on (use :0 for a random port)")
	webCmd.Flags().BoolVar(&webOpen, "open", true, "Open the interface in your default browser")
	webCmd.Flags().BoolVar(&webAllowRemote, "allow-remote", false, "Allow non-loopback Host headers (requires your own authentication proxy)")
	rootCmd.AddCommand(webCmd)
}
