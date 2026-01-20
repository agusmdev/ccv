package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// ClaudeRunner manages the Claude subprocess and message parsing
type ClaudeRunner struct {
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stdin      io.WriteCloser
	messages   chan interface{}
	errors     chan error
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewClaudeRunner creates a new Claude subprocess runner
func NewClaudeRunner(ctx context.Context, args []string) (*ClaudeRunner, error) {
	// Create cancellable context
	runnerCtx, cancel := context.WithCancel(ctx)

	// Build command args with required flags
	claudeArgs := []string{
		"--print",                   // Required for non-interactive mode
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
	}
	claudeArgs = append(claudeArgs, args...)

	// Create command
	cmd := exec.CommandContext(runnerCtx, "claude", claudeArgs...)

	// Get pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	runner := &ClaudeRunner{
		cmd:      cmd,
		stdout:   stdout,
		stderr:   stderr,
		stdin:    stdin,
		messages: make(chan interface{}, 100),
		errors:   make(chan error, 10),
		ctx:      runnerCtx,
		cancel:   cancel,
	}

	return runner, nil
}

// Start begins the Claude subprocess and starts parsing output
func (r *ClaudeRunner) Start() error {
	// Start the command
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// Close stdin immediately since we're in --print mode and don't need interactive input
	// This signals to Claude that no interactive input will be provided
	r.stdin.Close()

	// Start goroutine to parse stdout (NDJSON)
	r.wg.Add(1)
	go r.parseStdout()

	// Start goroutine to forward stderr
	r.wg.Add(1)
	go r.forwardStderr()

	// Start goroutine to wait for process completion
	r.wg.Add(1)
	go r.waitForCompletion()

	return nil
}

// parseStdout reads and parses NDJSON lines from stdout
func (r *ClaudeRunner) parseStdout() {
	defer r.wg.Done()
	defer close(r.messages)

	scanner := bufio.NewScanner(r.stdout)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse the JSON message
		msg, err := ParseMessage(line)
		if err != nil {
			r.errors <- fmt.Errorf("line %d: failed to parse message: %w", lineNum, err)
			continue
		}

		// Send parsed message to channel
		select {
		case r.messages <- msg:
		case <-r.ctx.Done():
			return
		}
	}

	if err := scanner.Err(); err != nil {
		r.errors <- fmt.Errorf("error reading stdout: %w", err)
	}
}

// forwardStderr forwards stderr to os.Stderr
func (r *ClaudeRunner) forwardStderr() {
	defer r.wg.Done()

	scanner := bufio.NewScanner(r.stderr)
	for scanner.Scan() {
		select {
		case <-r.ctx.Done():
			return
		default:
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		r.errors <- fmt.Errorf("error reading stderr: %w", err)
	}
}

// waitForCompletion waits for the process to complete
func (r *ClaudeRunner) waitForCompletion() {
	defer r.wg.Done()

	err := r.cmd.Wait()

	if err != nil {
		// Only report non-zero exit if context wasn't cancelled
		select {
		case <-r.ctx.Done():
			return
		default:
			r.errors <- fmt.Errorf("claude process exited: %w", err)
		}
	}
}

// Messages returns the channel for receiving parsed messages
func (r *ClaudeRunner) Messages() <-chan interface{} {
	return r.messages
}

// Errors returns the channel for receiving errors
func (r *ClaudeRunner) Errors() <-chan error {
	return r.errors
}

// Wait waits for all goroutines to complete
func (r *ClaudeRunner) Wait() {
	r.wg.Wait()
}

// Stop stops the runner and cancels the subprocess
func (r *ClaudeRunner) Stop() {
	r.cancel()
	r.Wait()
}

// WriteInput writes data to the subprocess stdin
func (r *ClaudeRunner) WriteInput(data []byte) error {
	_, err := r.stdin.Write(data)
	return err
}
