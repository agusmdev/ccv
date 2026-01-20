package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	version = "0.1.0"
)

func main() {
	// Define flags (only --help and --version)
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CCV - Claude Code Viewer\n\n")
		fmt.Fprintf(os.Stderr, "A headless CLI wrapper for Claude Code that outputs structured text.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ccv [prompt]\n")
		fmt.Fprintf(os.Stderr, "  ccv [claude args...]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  CCV_VERBOSE=1    Show verbose output including full tool inputs\n")
		fmt.Fprintf(os.Stderr, "  CCV_QUIET=1      Show only assistant text responses\n")
		fmt.Fprintf(os.Stderr, "  CCV_FORMAT=json  Output format: text (default), json\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ccv \"Explain this codebase\"\n")
		fmt.Fprintf(os.Stderr, "  ccv -p \"Fix the bug\" --allowedTools Bash,Read\n")
		fmt.Fprintf(os.Stderr, "  CCV_VERBOSE=1 ccv \"Debug this issue\"\n")
	}

	flag.Parse()

	// Read configuration from environment variables
	verbose := os.Getenv("CCV_VERBOSE") == "1"
	quiet := os.Getenv("CCV_QUIET") == "1"
	format := strings.ToLower(os.Getenv("CCV_FORMAT"))
	if format == "" {
		format = "text"
	}

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

	// Create output processor
	processor := NewOutputProcessor(format, verbose, quiet)

	// Process messages (blocks until completion)
	processor.ProcessMessages(runner.Messages(), runner.Errors())

	// Wait for runner to complete
	runner.Wait()
}
