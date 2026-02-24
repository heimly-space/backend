package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	http2 "net/http"
	"os/signal"
	"syscall"
	"time"

	"heimly.space/backend/internal/app"
	"heimly.space/backend/internal/cfg"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	printWelcome()

	println("📝 Loading Heimly config...")
	conf := cfg.Load()
	conf.PrintSummary()

	application := app.New(conf)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- application.HTTPServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		// Allow a repeated signal (Ctrl+C / IDE stop) to force-kill immediately.
		stop()
		fmt.Println("🛑 Shutdown signal received")
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http2.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
		closeDBWithTimeout(application.DB, 3*time.Second)
		printGoodbye()
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.HTTPServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v; forcing close", err)
		_ = application.HTTPServer.Close()
	}

	select {
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http2.ErrServerClosed) {
			log.Printf("http server exit error: %v", err)
		}
	case <-time.After(2 * time.Second):
		log.Printf("http server did not report shutdown in time; exiting")
	}

	closeDBWithTimeout(application.DB, 3*time.Second)
	printGoodbye()
}

func closeDBWithTimeout(dbCloser interface{ Close() }, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		dbCloser.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		log.Printf("db close timed out after %s; exiting", timeout)
	}
}
