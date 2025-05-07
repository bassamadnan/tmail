package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bassamadnan/tmail/config"
	"github.com/bassamadnan/tmail/gmail"
	"github.com/bassamadnan/tmail/tui"
)

const (
	filterConfigPath = "config/filters.json"
	initialPollDelay = 1 * time.Second  // Short delay for TUI to draw before initial emails
	pollInterval     = 30 * time.Second // How often to check for new emails via API
)

func main() {
	logFile, err := os.OpenFile("tmail.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Println("Application starting...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, cancelling context...")
		cancel()
	}()

	cfgManager, err := config.NewManager(filterConfigPath)
	if err != nil {
		log.Fatalf("Failed to initialize config manager: %v", err)
	}
	log.Println("Config manager initialized.")

	emailChan := make(chan gmail.ProcessedEmail, 15) // Buffer for initial emails + ongoing
	gmailClient, err := gmail.NewClient(ctx, cfgManager)
	if err != nil {
		log.Fatalf("Failed to initialize Gmail client: %v. Ensure credentials.json is present and valid.", err)
	}
	log.Println("Gmail client initialized.")

	go gmailClient.StartMonitoring(ctx, emailChan, initialPollDelay, pollInterval)
	log.Println("Gmail monitoring configured to start.")

	tuiApp := tui.NewApp(cfgManager, emailChan, pollInterval) // Pass pollInterval
	log.Println("TUI application initialized.")

	if err := tuiApp.Run(); err != nil {
		log.Fatalf("Error running TUI application: %v", err)
	}

	log.Println("TUI application stopped. Exiting.")
}
