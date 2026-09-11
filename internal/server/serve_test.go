package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/teandresmith/sqlgen-example/internal/server"
)

// TestServeDrainsInFlightRequest proves graceful shutdown (PRD story 58): a
// request already executing when shutdown is triggered runs to completion and
// receives its full response, and Serve does not return until that request has
// drained.
//
// The handler blocks until the test releases it, so the request is provably
// in-flight when the shutdown context is cancelled. The test then asserts Serve
// is still draining (has not returned) while the request is blocked, releases
// the handler, and checks that the client got a complete 200 response and Serve
// returned cleanly.
func TestServeDrainsInFlightRequest(t *testing.T) {
	addr := freeAddr(t)

	handlerReached := make(chan struct{})
	releaseHandler := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(handlerReached)
		<-releaseHandler
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("drained"))
	})
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(ctx, srv, 5*time.Second, slog.New(slog.DiscardHandler))
	}()
	waitListening(t, addr)

	// Fire a request and wait until it is provably executing in the handler.
	type clientResult struct {
		status int
		body   string
		err    error
	}
	reqDone := make(chan clientResult, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			reqDone <- clientResult{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		reqDone <- clientResult{status: resp.StatusCode, body: string(body), err: err}
	}()
	<-handlerReached

	// Trigger shutdown while the request is in-flight.
	cancel()

	// Serve must still be draining — it cannot return before the blocked
	// handler completes.
	select {
	case err := <-serveDone:
		t.Fatalf("Serve returned before in-flight request drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Let the in-flight handler finish; the client must get its full response.
	close(releaseHandler)

	res := <-reqDone
	if res.err != nil {
		t.Fatalf("in-flight request failed instead of draining: %v", res.err)
	}
	if res.status != http.StatusOK || res.body != "drained" {
		t.Fatalf("in-flight response = %d %q, want 200 %q", res.status, res.body, "drained")
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned error after clean drain: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after in-flight request drained")
	}
}

// TestServeReturnsListenError proves Serve surfaces a serve-time failure (e.g.
// the port already in use) rather than blocking forever waiting for a shutdown
// signal that will never resolve the fault.
func TestServeReturnsListenError(t *testing.T) {
	// Hold a listener open so the server's bind fails.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving addr: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Addr: ln.Addr().String(), Handler: http.NewServeMux()}
	err = server.Serve(context.Background(), srv, time.Second, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("Serve returned nil, want a bind error for an in-use address")
	}
}

// freeAddr reserves a free loopback port and returns its address. The listener
// is closed before returning; there is a small reuse window, tolerable for a
// test that immediately binds it.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// waitListening blocks until addr accepts TCP connections or the deadline
// passes, so the test only fires its request once the server is up.
func waitListening(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s", addr)
}
