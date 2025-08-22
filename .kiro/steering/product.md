# Product Overview

Slack MCP Server is a Model Context Protocol (MCP) server that provides AI assistants with comprehensive access to Slack workspaces. It's designed as a powerful integration tool that enables LLMs to interact with Slack data and functionality.

## Key Features

- **Dual Authentication**: Supports both stealth mode (xoxc/xoxd browser tokens) and OAuth mode (xoxp tokens)
- **Enterprise Support**: Works with Enterprise Slack setups and Slack Connect
- **Multiple Transports**: Supports both Stdio and SSE (Server-Sent Events) transports
- **Smart History**: Fetch messages with pagination by date ranges or message count
- **Search Capabilities**: Advanced message search with multiple filters (date, user, channel, content)
- **Channel Management**: Access to public/private channels, DMs, and group DMs
- **Safe Message Posting**: Optional message posting with channel restrictions for safety
- **Caching System**: Intelligent caching of users and channels for performance

## Target Users

- AI/LLM developers integrating Slack functionality
- Enterprise teams needing programmatic Slack access
- Developers building Slack-powered AI assistants
- Teams requiring stealth-mode Slack integration without bot permissions

## Safety & Security

The server prioritizes safety with disabled message posting by default, configurable channel restrictions, and support for proxy configurations for enterprise environments.