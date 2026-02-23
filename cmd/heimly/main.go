package main

import (
	"context"
	"errors"
	http2 "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"heimly.space/backend/internal/app"
	"heimly.space/backend/internal/cfg"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	printWelcome()

	println("📝 Loading Heimly config...")
	conf := cfg.Load()
	conf.PrintSummary()

	application := app.New(conf)
	defer application.DB.Close()

	go func() {
		if err := application.HTTPServer.ListenAndServe(); err != nil &&
			!errors.Is(err, http2.ErrServerClosed) {
			panic(err)
		}
	}()

	<-signalChan

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = application.HTTPServer.Shutdown(ctx)

	printGoodbye()
}
