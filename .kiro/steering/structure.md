# Project Structure

## Root Directory

- `cmd/` - Application entry points
- `pkg/` - Reusable packages and core logic
- `docs/` - Documentation files
- `npm/` - NPM packaging for distribution
- `build/` - Build artifacts and compiled binaries
- `images/` - Project assets (demos, icons)

## Core Application (`cmd/`)

```
cmd/slack-mcp-server/
├── main.go                 # Application entry point with transport selection
```

Main responsibilities:
- Command-line argument parsing (stdio vs sse transport)
- Logger configuration with environment-based settings
- Provider initialization and cache warming
- Server startup for chosen transport

## Package Structure (`pkg/`)

### `pkg/handler/` - MCP Tool Handlers
- `channels.go` - Channel listing and management
- `conversations.go` - Message history, replies, search, posting
- `*_test.go` - Unit tests for handlers

### `pkg/provider/` - Slack API Abstraction
- `api.go` - Main provider with authentication and caching
- `edge/` - Enterprise Slack client implementation
  - `client.go` - Edge API client
  - `conversations.go` - Enterprise conversation handling
  - `fasttime/` - Time utilities for performance

### `pkg/server/` - MCP Server Implementation
- `server.go` - MCP server setup with tools and resources
- `auth/` - Authentication middleware for SSE transport

### `pkg/transport/` - HTTP Transport Configuration
- `transport.go` - HTTP client setup with proxy support

### `pkg/text/` - Text Processing
- `text_processor.go` - Message formatting and workspace parsing

### `pkg/version/` - Version Information
- `version.go` - Build-time version injection

### `pkg/limiter/` - Rate Limiting
- `limits.go` - API rate limiting configuration

### `pkg/test/` - Testing Utilities
- `util/` - Test helpers for MCP and ngrok

## Naming Conventions

- **Files**: Snake case (e.g., `text_processor.go`)
- **Packages**: Single word, lowercase (e.g., `handler`, `provider`)
- **Types**: PascalCase (e.g., `ApiProvider`, `MCPServer`)
- **Functions**: PascalCase for exported, camelCase for private
- **Constants**: PascalCase or UPPER_CASE for package-level

## Testing Strategy

- Unit tests alongside source files (`*_test.go`)
- Integration tests with `Integration` suffix in test names
- Test utilities in `pkg/test/util/`
- Separate test commands: `make test` (unit) and `make test-integration`

## Configuration Files

- `.env.dist` - Environment variable template
- `docker-compose*.yml` - Container orchestration
- `Dockerfile` - Container build instructions
- `manifest-dxt.json` - DXT extension manifest