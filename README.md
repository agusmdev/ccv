# CCV - Claude Code Viewer

A lightweight TUI (Terminal User Interface) wrapper for Claude Code that renders beautiful, structured output in your terminal.

## Features

- **Beautiful TUI Interface**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for stunning terminal output
- **Real-time Streaming**: Watch Claude Code's responses stream in real-time with proper formatting
- **Tool Call Visualization**: See tool calls, results, and thinking blocks clearly structured
- **Message Tracking**: Track assistant messages, user inputs, and system events
- **Keyboard Navigation**: Mouse and keyboard support for interactive exploration
- **Signal Handling**: Graceful shutdown with proper cleanup

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

Use `--` to pass arguments directly to the underlying Claude Code CLI:

```bash
ccv -- -p "Fix the bug" --allowedTools Bash,Read
```

### Interactive Mode

The TUI provides:
- **Alt Screen**: Uses alternate screen buffer, preserving your terminal history
- **Mouse Support**: Click and scroll through content
- **Signal Handling**: Press `Ctrl+C` for graceful shutdown

### Examples

Analyze a codebase:
```bash
ccv "Analyze the architecture of this project"
```

Debug with specific tools:
```bash
ccv -- -p "Debug the authentication flow" --allowedTools Read,Grep
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

CCV respects your existing Claude Code configuration. Make sure Claude Code is properly configured:

```bash
claude auth login
```

For more Claude Code configuration options, see the [official documentation](https://docs.anthropic.com/en/docs/quickstart-guide).

## How It Works

CCV wraps the Claude Code CLI and:
1. Spawns Claude Code as a subprocess with your prompt
2. Parses the streaming JSON output from Claude Code's SDK
3. Renders structured messages, tool calls, and results in a beautiful TUI
4. Handles signals and cleanup for graceful shutdown

The TUI displays:
- **Assistant Messages**: Claude's responses with proper formatting
- **Tool Calls**: Function calls with parameters
- **Tool Results**: Results from executed tools
- **Thinking Blocks**: Claude's reasoning process
- **User Messages**: Your inputs and system messages

## Development

### Project Structure

```
ccv/
├── main.go      # Entry point and flag handling
├── runner.go    # Claude Code subprocess management
├── tui.go       # Bubble Tea UI implementation
├── types.go     # Message and event type definitions
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

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful TUI framework
- Styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions for nice terminal layouts
- Wraps [Claude Code](https://docs.anthropic.com/en/docs/quickstart-guide) - Anthropic's official CLI for Claude

## Support

If you encounter any issues or have questions:
- Open an issue on [GitHub](https://github.com/agusmdev/ccv/issues)
- Check the [Claude Code documentation](https://docs.anthropic.com/en/docs/quickstart-guide)
- Make sure Claude Code CLI is properly installed and configured
