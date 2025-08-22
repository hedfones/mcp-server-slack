# Project Structure

## Directory Organization

```
├── cmd/slack-mcp-server/     # Main application entry point
├── pkg/                      # Core application packages
│   ├── handler/             # MCP tool handlers (channels, conversations)
│   ├── limiter/             # Rate limiting functionality
│   ├── provider/            # Slack API provider and edge client
│   │   └── edge/           # Low-level Slack API client (from rusq/slackdump)
│   ├── server/             # HTTP server and middleware
│   │   ├── auth/           # SSE authentication
│   │   └── middleware/     # Security middleware
│   ├── text/               # Text processing utilities
│   ├── transport/          # MCP transport layer
│   └── version/            # Version information
├── docs/                    # Documentation
├── npm/                     # NPM package distributions
├── build/                   # Build artifacts
└── images/                  # Project assets
```

## Package Responsibilities

### cmd/slack-mcp-server
- Main application entry point
- Configuration loading and validation
- Transport selection (stdio/sse)
- Logging setup
- Cache warming (users/channels)

### pkg/handler
- **channels.go**: Channel listing and management tools
- **conversations.go**: Message history, replies, search, and posting tools
- Implements MCP tool interface

### pkg/provider
- **api.go**: Main API provider interface
- **edge/**: Low-level Slack client (adapted from rusq/slackdump)
- Handles authentication, caching, and Slack API interactions

### pkg/server
- **server.go**: MCP server implementation
- **health.go**: Health check endpoints
- **middleware/**: Security headers, rate limiting
- **auth/**: SSE authentication

## Code Organization Patterns

### Configuration
- Environment variables loaded in main.go
- Validation separated from loading
- Railway-specific deployment handling

### Error Handling
- Structured logging with zap
- Context-aware error propagation
- Graceful degradation for missing features

### Testing
- Unit tests alongside source files (*_test.go)
- Integration tests in separate test runs
- Test utilities in pkg/test/

### Build Artifacts
- Multi-platform binaries in build/
- NPM packages for different architectures
- Docker images for containerized deployment

## Naming Conventions

- **Files**: snake_case for Go files
- **Packages**: lowercase, descriptive names
- **Functions**: PascalCase for exported, camelCase for internal
- **Constants**: UPPER_SNAKE_CASE for environment variables
- **Interfaces**: Descriptive names ending in -er when appropriate