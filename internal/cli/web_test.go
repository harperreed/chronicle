// ABOUTME: Tests for how the web command serves and shuts down.
// ABOUTME: They run a real HTTP server on a loopback port.

package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

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
