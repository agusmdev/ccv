package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var (
	version = "0.1.0"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, "CCV - Claude Code Viewer\n\n")
	fmt.Fprintf(os.Stderr, "A headless CLI wrapper for Claude Code that outputs structured text.\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  ccv [options] [prompt]\n")
	fmt.Fprintf(os.Stderr, "  ccv [options] [claude args...]\n")
	fmt.Fprintf(os.Stderr, "  ccv --history <path>             View past session transcripts\n\n")
	fmt.Fprintf(os.Stderr, "Output Flags:\n")
	fmt.Fprintf(os.Stderr, "  --verbose        Show verbose output including full tool inputs\n")
	fmt.Fprintf(os.Stderr, "  --quiet          Show only assistant text responses\n")
	fmt.Fprintf(os.Stderr, "  --format <fmt>   Output format: text (default), json\n")
	fmt.Fprintf(os.Stderr, "  --no-color       Disable colored output\n")
	fmt.Fprintf(os.Stderr, "\nHistory Mode Flags:\n")
	fmt.Fprintf(os.Stderr, "  --history <path> Read past sessions from path (file or directory)\n")
	fmt.Fprintf(os.Stderr, "  --since <date>   Filter sessions modified since date (YYYY-MM-DD)\n")
	fmt.Fprintf(os.Stderr, "  --last <n>       Show only the last N sessions\n")
	fmt.Fprintf(os.Stderr, "  --project <name> Find project by name in ~/.claude/projects/\n")
	fmt.Fprintf(os.Stderr, "\nGeneral Flags:\n")
	fmt.Fprintf(os.Stderr, "  --help           Show help information\n")
	fmt.Fprintf(os.Stderr, "  --version        Show version information\n")
	fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
	fmt.Fprintf(os.Stderr, "  CCV_VERBOSE=1    Equivalent to --verbose\n")
	fmt.Fprintf(os.Stderr, "  CCV_QUIET=1      Equivalent to --quiet\n")
	fmt.Fprintf(os.Stderr, "  CCV_FORMAT=json  Equivalent to --format json\n")
	fmt.Fprintf(os.Stderr, "  NO_COLOR=1       Disable colored output (standard)\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  ccv \"Explain this codebase\"\n")
	fmt.Fprintf(os.Stderr, "  ccv --verbose \"Debug this issue\"\n")
	fmt.Fprintf(os.Stderr, "  ccv --format json \"List all files\"\n")
	fmt.Fprintf(os.Stderr, "  ccv -p \"Fix the bug\" --allowedTools Bash,Read\n")
	fmt.Fprintf(os.Stderr, "\nHistory Examples:\n")
	fmt.Fprintf(os.Stderr, "  ccv --history ~/.claude/projects/-Users-me-myproject/\n")
	fmt.Fprintf(os.Stderr, "  ccv --history --last 5 ~/.claude/projects/-Users-me-myproject/\n")
	fmt.Fprintf(os.Stderr, "  ccv --history --project myproject\n")
	fmt.Fprintf(os.Stderr, "  ccv --history --since 2025-01-20 --project myproject\n")
}

func main() {
	// Initialize colors based on terminal capability
	initColors()

	// Manually parse only ccv's own flags to allow passthrough of all other args to claude
	// This avoids Go's flag package rejecting unknown flags like --model
	args := os.Args[1:]

	// Read configuration from environment variables (can be overridden by flags)
	verbose := os.Getenv("CCV_VERBOSE") == "1"
	quiet := os.Getenv("CCV_QUIET") == "1"
	noColor := false
	format := strings.ToLower(os.Getenv("CCV_FORMAT"))
	if format == "" {
		format = "text"
	}

	// History mode flags
	var historyPath string
	var historySince string
	var historyLast int
	var historyProject string
	historyMode := false

	// Parse and filter ccv-specific flags, pass remaining args to claude
	var claudeArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-version" || arg == "--version" {
			fmt.Printf("ccv version %s\n", version)
			os.Exit(0)
		}
		if arg == "-help" || arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--verbose" || arg == "-verbose" {
			verbose = true
			continue
		}
		if arg == "--quiet" || arg == "-quiet" {
			quiet = true
			continue
		}
		if arg == "--no-color" || arg == "-no-color" {
			noColor = true
			continue
		}
		if arg == "--format" || arg == "-format" {
			// Next arg is the format value
			if i+1 < len(args) {
				i++
				format = strings.ToLower(args[i])
			}
			continue
		}
		// Handle --format=value syntax
		if strings.HasPrefix(arg, "--format=") {
			format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
			continue
		}
		if strings.HasPrefix(arg, "-format=") {
			format = strings.ToLower(strings.TrimPrefix(arg, "-format="))
			continue
		}

		// History mode flags
		if arg == "--history" || arg == "-history" {
			historyMode = true
			// Check if next arg is a path (doesn't start with -)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				historyPath = args[i]
			}
			continue
		}
		if strings.HasPrefix(arg, "--history=") {
			historyMode = true
			historyPath = strings.TrimPrefix(arg, "--history=")
			continue
		}
		if arg == "--since" || arg == "-since" {
			if i+1 < len(args) {
				i++
				historySince = args[i]
			}
			continue
		}
		if strings.HasPrefix(arg, "--since=") {
			historySince = strings.TrimPrefix(arg, "--since=")
			continue
		}
		if arg == "--last" || arg == "-last" {
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					historyLast = n
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "--last=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--last=")); err == nil {
				historyLast = n
			}
			continue
		}
		if arg == "--project" || arg == "-project" {
			historyMode = true
			if i+1 < len(args) {
				i++
				historyProject = args[i]
			}
			continue
		}
		if strings.HasPrefix(arg, "--project=") {
			historyMode = true
			historyProject = strings.TrimPrefix(arg, "--project=")
			continue
		}

		// Pass through to claude
		claudeArgs = append(claudeArgs, arg)
	}

	// Apply --no-color flag to color system
	if noColor {
		SetNoColor(true)
	}

	// Handle history mode
	if historyMode {
		// If we still don't have a path but have remaining args, use the first one
		if historyPath == "" && historyProject == "" && len(claudeArgs) > 0 {
			historyPath = claudeArgs[0]
		}

		if historyPath == "" && historyProject == "" {
			fmt.Fprintln(os.Stderr, "Error: --history requires a path or --project name")
			printUsage()
			os.Exit(1)
		}

		reader := NewHistoryReader()
		opts := HistoryOptions{
			Path:    historyPath,
			Since:   historySince,
			Last:    historyLast,
			Project: historyProject,
		}

		if err := reader.Run(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	args = claudeArgs
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No prompt or arguments provided")
		printUsage()
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
