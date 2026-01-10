package main

import (
	"os"
	"os/signal"
	"syscall"

	"heimly.space/backend/internal/cfg"
	db2 "heimly.space/backend/internal/db"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	printWelcome()

	println("📝 Loading Heimly config...")
	conf := cfg.Load()
	conf.PrintSummary()

	println("Connecting to database...")
	pool := db2.ConnectDB(conf)
	defer pool.Close()

	println("Running migrations...")
	db2.RunMigrations(pool)

	<-signalChan
	printGoodbye()
}
