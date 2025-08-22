# Technology Stack

## Core Technologies

- **Language**: Go 1.24.4
- **Framework**: Model Context Protocol (MCP) using `github.com/mark3labs/mcp-go`
- **Slack Integration**: `github.com/rusq/slack` and `github.com/slack-go/slack`
- **Authentication**: `github.com/rusq/slackauth` for token management
- **Logging**: `go.uber.org/zap` for structured logging
- **HTTP**: Standard library with custom middleware for security and rate limiting

## Key Dependencies

- **MCP Protocol**: `github.com/mark3labs/mcp-go` - Core MCP server implementation
- **Slack APIs**: Multiple Slack client libraries for different authentication methods
- **Security**: Custom TLS handling with `github.com/refraction-networking/utls`
- **CSV Processing**: `github.com/gocarina/gocsv` for resource exports
- **Testing**: `github.com/stretchr/testify` for unit tests

## Build System

### Common Commands

```bash
# Build for current platform
make build

# Build for all platforms (darwin, linux, windows on amd64/arm64)
make build-all-platforms

# Run tests
make test

# Run integration tests  
make test-integration

# Format code
make format

# Clean build artifacts
make clean

# Create release tag
make release TAG=v1.2.3
```

### Development Workflow

- Use `make format` before committing
- Run `make test` for unit tests
- Use `make tidy` to clean up go.mod
- Build with `make build` for local testing

## Transport Modes

- **Stdio**: For direct MCP client integration (default)
- **SSE**: HTTP Server-Sent Events for web-based clients
- Configurable via `--transport` flag or `-t` shorthand

## Environment Configuration

Extensive environment variable configuration for:
- Slack authentication (XOXC/XOXD tokens or XOXP OAuth)
- Server settings (host, port, CORS, rate limiting)
- Security features (headers, health checks, proxy support)
- Caching (users, channels)
- Logging (level, format, colors)