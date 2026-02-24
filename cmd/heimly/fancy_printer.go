package main

import "fmt"

func printWelcome() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✨ Heimly backend starting")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func printGoodbye() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("👋 Heimly backend stopped")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func printStep(message string) {
	fmt.Printf("📝 %s\n", message)
}

func printInfo(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}

func printOK(message string) {
	fmt.Printf("✅ %s\n", message)
}

func printWarn(message string) {
	fmt.Printf("⚠️  %s\n", message)
}
