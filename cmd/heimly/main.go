package main

import (
	"os"
	"os/signal"
	"syscall"

	"heimly.space/backend/internal/cfg"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	printWelcome()

	println("📝 Loading Heimly config...")
	conf := cfg.Load()
	conf.PrintSummary()

	<-signalChan
	printGoodbye()
}
