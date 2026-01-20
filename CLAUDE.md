# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CCV (Claude Code Viewer) is a Go-based headless CLI wrapper for the Claude Code CLI. It parses the streaming JSON output from `claude --output-format stream-json` and renders structured text output showing assistant responses, tool calls, thinking blocks, and results. Designed for use in scripts, automation, logging, and headless environments where an interactive TUI is not suitable.

## Development Commands

### Building
```bash
go build -o ccv
```

### Running Locally
```bash
go run . "Your prompt here"
go run . -- -p "Your prompt" --allowedTools Read,Bash
```

### Installing
```bash
# Build and install to system
go build -o ccv
sudo mv ccv /usr/local/bin/

# Or use Go install
go install github.com/agusmdev/ccv@latest
```

## Architecture

### Core Components

**main.go** - Entry point
- Flag parsing (--version, --help, --verbose, --quiet, --format)
- Context creation with signal handling (SIGINT, SIGTERM)
- Initializes ClaudeRunner and headless output processor
- Outputs structured text to stdout, suitable for piping and logging

**runner.go** - Claude subprocess management
- Spawns `claude` CLI with `--output-format stream-json --include-partial-messages`
- Manages stdin/stdout/stderr pipes
- Parses NDJSON (newline-delimited JSON) output
- Goroutine-based concurrent processing:
  - `parseStdout()` - Parses streaming JSON messages
  - `forwardStderr()` - Forwards errors to os.Stderr
  - `forwardStdin()` - Forwards stdin for permission prompts
  - `waitForCompletion()` - Monitors process exit

**types.go** - Message type definitions and state management
- Defines SDK message types: `SystemInit`, `AssistantMessage`, `StreamEvent`, `Result`
- Content block types: `text`, `tool_use`, `tool_result`, `thinking`, `redacted_thinking`
- `AppState` - Central state management:
  - Agent hierarchy tracking (root agent, current agent, children)
  - Tool call tracking (pending, running, completed, failed)
  - Token usage accumulation
  - Streaming state for partial messages
- `ParseMessage()` - Parses JSON into appropriate type structs

**output.go** (TO BE CREATED) - Text output processor
- Consumes messages from `runner.messages` channel
- Formats and writes structured text to stdout in real-time
- Output includes:
  - Assistant text responses (streamed as they arrive)
  - Thinking blocks with `[THINKING]` prefix
  - Tool calls with format: `→ ToolName: <description> [status]`
  - Tool results (indented under tool calls)
  - Agent context: `[agent_type: status]`
  - Token usage summary at completion
- Supports output modes:
  - Default: Structured text with moderate detail
  - `--verbose`: Include full tool inputs and parameters
  - `--quiet`: Only show assistant text responses
  - `--format json`: Output parsed SDK messages as JSON

### Data Flow

1. User provides prompt → `main.go` parses args and flags
2. `NewClaudeRunner()` spawns `claude` subprocess with streaming flags
3. `runner.parseStdout()` reads NDJSON lines, calls `ParseMessage()`
4. Parsed messages sent to `runner.messages` channel
5. Output processor goroutine receives messages from channel
6. Messages are processed to update `AppState`
7. Text formatter outputs structured text to stdout in real-time

### State Management

The `AppState` struct is the single source of truth:
- **Agent Hierarchy**: Tracks nested Task tool calls as child agents
- **Tool Tracking**: Maps tool call IDs to ToolCall structs with status
- **Token Counting**: Accumulates input/output/cache tokens
- **Stream State**: Holds partial text/thinking/tool input during streaming

When a `StreamEvent` arrives with `ContentBlockDelta`, the partial content accumulates in `StreamState`. When an `AssistantMessage` arrives, streaming state is cleared and the complete message is processed.

### Agent Context Display

Agent context is shown inline in the text output:
- `[main: running]` - Root agent executing
- `[task: running]` - Task subagent active
- Nested agents are indented to show hierarchy
- Agent transitions are logged as they occur

## Issue Tracking with bd (beads)

This project uses **bd** (beads) for issue tracking. The `.beads/` directory contains the SQLite database.

### Common bd Commands
```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Session Completion Workflow

When ending a work session:

1. **File issues** for remaining work
2. **Run quality gates** (if code changed) - Build with `go build -o ccv`
3. **Update issue status** - Close finished work with `bd close <id>`
4. **PUSH TO REMOTE** (MANDATORY):
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL**: Work is NOT complete until `git push` succeeds. Never say "ready to push when you are" - YOU must push.

## Key Technical Details

### Message Parsing
- Scanner buffer increased to 1MB (`scanner.Buffer(buf, 1024*1024)`) to handle large tool results
- Empty lines are skipped
- Parse errors are sent to `runner.errors` channel but don't stop processing

### Concurrent Goroutines
- All goroutines use `sync.WaitGroup` for graceful shutdown
- Context cancellation propagates to subprocess via `exec.CommandContext`
- Channels are buffered (messages: 100, errors: 10) to prevent blocking

### Streaming Text Output
- Assistant text is streamed to stdout as it arrives (real-time display)
- Thinking blocks are displayed with `[THINKING]` prefix
- Tool calls show name, status, and description
- Tool results are indented under their corresponding tool calls
- Token counts are displayed incrementally as usage deltas arrive
- Final summary shows total tokens, cost, and duration
