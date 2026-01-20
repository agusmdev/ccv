package main

import (
	"flag"
	"fmt"
	"os"
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

	// TODO: Initialize TUI and start Claude subprocess
	fmt.Printf("Starting CCV with args: %v\n", args)
}
