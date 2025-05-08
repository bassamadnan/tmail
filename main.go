package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bassamadnan/tmail/config"
	"github.com/bassamadnan/tmail/gmail"
	"github.com/bassamadnan/tmail/tui" // Updated import
	tea "github.com/charmbracelet/bubbletea"
)

const (
	filterConfigPath = "config/filters.json"
	initialPollDelay = 1 * time.Second  // Short delay before initial emails
	pollInterval     = 30 * time.Second // How often to check for new emails via API
)

func main() {
	logFile, err := os.OpenFile("tmail.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile) // Send default logger to file
	// Also send Bubble Tea's default logger to the file if debugging is needed
	// if f, err := tea.LogToFile("bubbletea-debug.log", "debug"); err == nil {
	// 	defer f.Close()
	// } else {
	//  log.Printf("could not open bubbletea log file: %v", err)
	// }

	log.Println("Application starting...")

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cfgManager, err := config.NewManager(filterConfigPath)
	if err != nil {
		log.Fatalf("Failed to initialize config manager: %v", err)
	}
	log.Println("Config manager initialized.")

	emailChan := make(chan gmail.ProcessedEmail, 25) // Increased buffer slightly
	gmailClient, err := gmail.NewClient(appCtx, cfgManager)
	if err != nil {
		log.Fatalf("Failed to initialize Gmail client: %v. Ensure credentials.json is present and valid.", err)
	}
	log.Println("Gmail client initialized.")

	// Start Gmail monitoring in a goroutine. It will send emails to emailChan.
	// The Bubble Tea app will listen to this channel via a command.
	go func() {
		log.Println("Gmail monitoring goroutine configured to start.")
		gmailClient.StartMonitoring(appCtx, emailChan, initialPollDelay, pollInterval)
		log.Println("Gmail monitoring goroutine finished.")
		close(emailChan) // Close channel when monitoring stops
	}()

	// Pass pollInterval for display purposes in status bar
	initialModel := tui.NewInitialModel(cfgManager, emailChan, pollInterval)
	p := tea.NewProgram(initialModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Handle shutdown signals for the Bubble Tea program
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, sending quit to Bubble Tea program and cancelling context...")
		cancelApp() // Signal Gmail monitor and other potential context-aware goroutines
		p.Quit()    // Gracefully stop Bubble Tea
	}()

	log.Println("TUI application starting...")
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI application: %v", err)
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	log.Println("TUI application stopped. Exiting.")
}
