# Changelog

All notable changes to CCV will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2025-01-22

### Fixed

- Fix install.sh checksum verification on macOS (BSD sha256sum doesn't support -c flag)

## [0.1.0] - 2025-01-22

### Added

- Initial release of CCV (Claude Code Viewer)
- Headless CLI wrapper for Claude Code with structured text output
- Real-time streaming of assistant responses
- Tool call rendering with status indicators
- Thinking block display with `[THINKING]` prefix
- Support for 40+ tool types including:
  - File operations (Read, Write, Edit, Glob, Grep)
  - Browser automation (Playwright tools)
  - MCP tools (Context7, custom servers)
  - Task management (TodoWrite, Task agents)
  - Plan mode tools (EnterPlanMode, ExitPlanMode)
- Output modes: default, `--verbose`, `--quiet`, `--format json`
- Color output with `--no-color` option
- History mode (`--history`) for viewing past Claude sessions
- Token usage tracking and cost estimation
- Cross-platform support (macOS, Linux, Windows)

[0.1.1]: https://github.com/agusmdev/ccv/releases/tag/v0.1.1
[0.1.0]: https://github.com/agusmdev/ccv/releases/tag/v0.1.0
