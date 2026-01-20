package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	version = "0.1.0"
)

func main() {
	// Define flags
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CCV - Claude Code Viewer\n\n")
		fmt.Fprintf(os.Stderr, "A lightweight TUI wrapper for Claude Code that renders beautiful output.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ccv [flags] [prompt]\n")
		fmt.Fprintf(os.Stderr, "  ccv [flags] -- [claude args...]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ccv \"Explain this codebase\"\n")
		fmt.Fprintf(os.Stderr, "  ccv -- -p \"Fix the bug\" --allowedTools Bash,Read\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("ccv version %s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No prompt or arguments provided")
		flag.Usage()
		os.Exit(1)
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Create and start the Claude runner
	runner, err := NewClaudeRunner(ctx, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating runner: %v\n", err)
		os.Exit(1)
	}

	if err := runner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting Claude: %v\n", err)
		os.Exit(1)
	}

	// Process messages and errors
	go func() {
		for msg := range runner.Messages() {
			// TODO: Send to TUI for rendering
			// For now, just print the type
			switch m := msg.(type) {
			case *SystemInit:
				fmt.Printf("[SYSTEM] Session: %s, Model: %s\n", m.SessionID, m.Model)
			case *AssistantMessage:
				fmt.Printf("[ASSISTANT] Message received\n")
			case *Result:
				fmt.Printf("[RESULT] %s (Total cost: $%.4f)\n", m.Result, m.TotalCost)
			case *StreamEvent:
				fmt.Printf("[STREAM] %s\n", m.Type)
			default:
				fmt.Printf("[MESSAGE] Type: %T\n", msg)
			}
		}
	}()

	go func() {
		for err := range runner.Errors() {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}()

	// Wait for runner to complete
	runner.Wait()
}
