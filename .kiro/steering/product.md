# Product Overview

Slack MCP Server is a Model Context Protocol (MCP) server that provides AI assistants with comprehensive access to Slack workspaces. It enables reading messages, searching conversations, posting messages, and accessing channel/user information through a standardized MCP interface.

## Key Features

- **Dual Authentication**: Supports both stealth mode (browser tokens) and OAuth tokens
- **Multiple Transports**: Stdio and SSE (Server-Sent Events) transport protocols
- **Smart History**: Fetch messages by date ranges or message count with pagination
- **Search Capabilities**: Advanced message search with filters (date, user, channel, content)
- **Channel Management**: Access to public/private channels, DMs, and group DMs
- **Message Posting**: Safe message posting with optional channel restrictions
- **Enterprise Support**: Works with Enterprise Slack setups and proxy configurations
- **Caching**: User and channel caching for improved performance
- **Security**: Rate limiting, CORS, security headers, and health checks

## Target Use Cases

- AI assistants analyzing Slack conversations and workspace activity
- Automated message posting and thread management
- Workspace analytics and reporting
- Integration with MCP-compatible AI clients (Claude, etc.)
- Enterprise Slack automation and monitoring