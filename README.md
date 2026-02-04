# CCV - Claude Code Viewer

> **Warning**: This project is under active development. APIs and output formats may change between versions. Use with caution in production scripts.

Renders Claude Code's streaming JSON as readable terminal output. A drop-in replacement for headless environments where the interactive TUI isn't available.

<img width="777" height="692" alt="image" src="https://github.com/user-attachments/assets/77d5f72e-d546-4671-af80-dbabc78c885f" />


## Drop-in Replacement for Headless Scripts

**CCV is designed as a drop-in replacement for `claude` in headless environments.** Simply replace `claude` with `ccv` in your existing scripts to get pretty, structured output instead of raw JSON:

```bash
# Before (raw stream-json output)
claude --print -p "Explain this code" --output-format stream-json

# After (pretty formatted output)
ccv "Explain this code"
```

CCV automatically handles all the necessary flags (`--print`, `--output-format stream-json`, `--verbose`) so you can focus on your prompts. All Claude CLI arguments are passed through transparently.

## Features

- **Structured Text Output**: Clean, readable output showing assistant responses, tool calls, and thinking blocks
- **Real-time Streaming**: Claude's responses stream to stdout as they arrive
- **Tool Call Formatting**: Tool calls and results are clearly formatted with status indicators
- **Agent Hierarchy Display**: Shows nested agent context when Task tools spawn sub-agents
- **Token Usage Tracking**: Displays token counts and cost information
- **Headless Operation**: No interactive UI - perfect for scripts, CI/CD, and automation
- **Flexible Output Modes**: Supports verbose, quiet, and JSON output formats
- **Signal Handling**: Graceful shutdown with proper cleanup on SIGINT/SIGTERM

## Requirements

- Go 1.22 or later (for building from source)
- [Claude Code CLI](https://docs.anthropic.com/en/docs/quickstart-guide) must be installed and configured

## Installation

### One-Liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/agusmdev/ccv/main/install.sh | bash
```

This will automatically:
- Detect your OS (macOS, Linux) and architecture (amd64, arm64)
- Download the appropriate binary from GitHub releases
- Install to `/usr/local/bin` (or `~/.local/bin` if no sudo access)
- Verify checksums for security

### Using Go Install

If you have Go installed, you can install directly:

```bash
go install github.com/agusmdev/ccv@latest
```

### Manual Binary Download

1. Visit the [releases page](https://github.com/agusmdev/ccv/releases)
2. Download the binary for your OS and architecture
3. Make it executable: `chmod +x ccv`
4. Move to your PATH: `sudo mv ccv /usr/local/bin/`

### Build from Source

```bash
git clone https://github.com/agusmdev/ccv.git
cd ccv
make install
```

Or manually:

```bash
go build -o ccv
sudo mv ccv /usr/local/bin/
```

## Usage

### Basic Usage

Pass your prompt directly to `ccv`:

```bash
ccv "Explain this codebase"
```

### With Claude Code Arguments

Pass Claude Code arguments directly - CCV filters its own flags and passes everything else through:

```bash
ccv -p "Fix the bug" --allowedTools Bash,Read
```

### Output Modes

Control output verbosity with flags:

```bash
# Default: structured text with moderate detail
ccv "Explain this codebase"

# Verbose: include full tool inputs and parameters
ccv --verbose "Refactor this code"

# Quiet: only show assistant text responses
ccv --quiet "What is this project?"

# JSON: output parsed SDK messages as JSON
ccv --format json "Analyze the code"

# Disable colors (useful for logging or piping)
ccv --no-color "List all files"
```

### Piping and Scripting

CCV outputs to stdout, making it perfect for piping:

```bash
# Save output to a log file
ccv "Analyze the code" > analysis.log

# Pipe to other tools
ccv "List all functions" | grep "export"

# Use in scripts
OUTPUT=$(ccv --quiet "What is the main entry point?")
echo "Entry point: $OUTPUT"
```

### Examples

Analyze a codebase:
```bash
ccv "Analyze the architecture of this project"
```

Debug with specific tools:
```bash
ccv -p "Debug the authentication flow" --allowedTools Read,Grep
```

Get help:
```bash
ccv --help
```

Check version:
```bash
ccv --version
```

## Configuration

### CCV Flags

| Flag | Description |
|------|-------------|
| `--verbose` | Show verbose output including full tool inputs |
| `--quiet` | Show only assistant text responses |
| `--format <fmt>` | Output format: `text` (default) or `json` |
| `--no-color` | Disable colored output |
| `--help` | Show help information |
| `--version` | Show version information |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CCV_VERBOSE=1` | Equivalent to `--verbose` |
| `CCV_QUIET=1` | Equivalent to `--quiet` |
| `CCV_FORMAT=json` | Equivalent to `--format json` |
| `NO_COLOR=1` | Disable colored output (standard [no-color.org](https://no-color.org/)) |
| `TERM=dumb` | Also disables colored output |

### Claude Code Configuration

CCV respects your existing Claude Code configuration. Make sure Claude Code is properly configured:

```bash
claude auth login
```

For more Claude Code configuration options, see the [official documentation](https://docs.anthropic.com/en/docs/quickstart-guide).

## How It Works

CCV wraps the Claude Code CLI and:
1. Spawns Claude Code as a subprocess with `--output-format stream-json`
2. Parses the streaming NDJSON output from Claude Code's SDK
3. Formats and outputs structured text to stdout in real-time
4. Handles signals and cleanup for graceful shutdown

The output includes:
- **Assistant Messages**: Claude's text responses streamed as they arrive
- **Tool Calls**: Function calls with name, description, and status
- **Tool Results**: Results from executed tools (indented under tool calls)
- **Thinking Blocks**: Claude's reasoning process with `[THINKING]` prefix
- **Agent Context**: Shows current agent type and status (e.g., `[main: running]`)
- **Token Usage**: Token counts and cost summary at completion

## Development

### Project Structure

```
ccv/
├── main.go      # Entry point and flag handling
├── runner.go    # Claude Code subprocess management
├── output.go    # Text output processor and message formatting
├── types.go     # Message and event type definitions
├── colors.go    # Terminal color scheme and ANSI codes
├── format.go    # Text formatting utilities
└── go.mod       # Go module dependencies
```

### Running Locally

```bash
go run . "Your prompt here"
```

### Building

```bash
go build -o ccv
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Wraps [Claude Code](https://docs.anthropic.com/en/docs/quickstart-guide) - Anthropic's official CLI for Claude
- Built with Go's standard library for robust subprocess management and concurrent processing

## Support

If you encounter any issues or have questions:
- Open an issue on [GitHub](https://github.com/agusmdev/ccv/issues)
- Check the [Claude Code documentation](https://docs.anthropic.com/en/docs/quickstart-guide)
- Make sure Claude Code CLI is properly installed and configured
