## 4. Railway Deployment

This guide covers deploying the Slack MCP Server on Railway.app with IPv6 support and remote MCP server capabilities.

### Overview

Railway deployment enables you to run the Slack MCP Server as a cloud-hosted service accessible over the internet. This expands beyond local-only usage while maintaining security and performance through:

- **IPv6 Dual-Stack Support**: Automatic IPv4/IPv6 binding for modern network environments
- **Health Monitoring**: Built-in health checks for Railway's monitoring systems
- **Security Features**: CORS, rate limiting, and security headers for safe remote access
- **Auto-Configuration**: Automatic detection of Railway environment with sensible defaults

### Quick Start

1. **Fork or Clone the Repository**
   ```bash
   git clone https://github.com/korotovsky/slack-mcp-server.git
   cd slack-mcp-server
   ```

2. **Deploy to Railway**
   - Connect your GitHub repository to Railway
   - Railway will automatically detect the `railway.toml` configuration
   - Set your environment variables (see [Environment Variables](#environment-variables))
   - Deploy using the existing Dockerfile

3. **Configure Your MCP Client**
   ```json
   {
     "mcpServers": {
       "slack": {
         "command": "npx",
         "args": [
           "-y",
           "mcp-remote",
           "https://your-app.railway.app/sse",
           "--header",
           "Authorization: Bearer ${SLACK_MCP_SSE_API_KEY}"
         ],
         "env": {
           "SLACK_MCP_SSE_API_KEY": "your-secure-api-key"
         }
       }
     }
   }
   ```

### Railway Configuration

The project includes a `railway.toml` file that configures:

```toml
[build]
builder = "dockerfile"
dockerfilePath = "Dockerfile"

[deploy]
healthcheckPath = "/health"
healthcheckTimeout = 30
restartPolicyType = "on_failure"
```

### Environment Variables

#### Railway-Specific Variables

These are automatically set by Railway:

| Variable | Description |
|----------|-------------|
| `PORT` | Railway-provided port (overrides `SLACK_MCP_PORT`) |
| `RAILWAY_ENVIRONMENT` | Railway environment name (production, staging, etc.) |

#### Required Authentication Variables

| Variable | Required? | Description |
|----------|-----------|-------------|
| `SLACK_MCP_XOXC_TOKEN` | Yes* | Slack browser token (`xoxc-...`) |
| `SLACK_MCP_XOXD_TOKEN` | Yes* | Slack browser cookie `d` (`xoxd-...`) |
| `SLACK_MCP_XOXP_TOKEN` | Yes* | User OAuth token (`xoxp-...`) — alternative to xoxc/xoxd |
| `SLACK_MCP_SSE_API_KEY` | Yes | Bearer token for SSE transport authentication |

*Either use `SLACK_MCP_XOXP_TOKEN` OR both `SLACK_MCP_XOXC_TOKEN` and `SLACK_MCP_XOXD_TOKEN`

#### Network Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SLACK_MCP_HOST` | `""` (dual-stack) | Host binding (empty for IPv4/IPv6 dual-stack) |
| `SLACK_MCP_PORT` | `8080` | Fallback port when Railway PORT not available |
| `SLACK_MCP_BASE_URL` | Auto-detected | External base URL for Railway deployment |

#### Security Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SLACK_MCP_CORS_ORIGINS` | `"*"` | Comma-separated allowed CORS origins |
| `SLACK_MCP_RATE_LIMIT` | `60` | Requests per minute per IP address |
| `SLACK_MCP_SECURITY_HEADERS` | `true` | Enable security headers |
| `SLACK_MCP_HEALTH_ENABLED` | `true` | Enable health check endpoints |

#### Logging Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SLACK_MCP_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `SLACK_MCP_LOG_FORMAT` | `json` (Railway) | Log format: json or console |
| `SLACK_MCP_LOG_COLOR` | `false` (Railway) | Enable colored console output |

### IPv6 Configuration

The server automatically supports IPv6 dual-stack networking when deployed on Railway:

#### Automatic IPv6 Detection

- **Railway Environment**: Automatically enables dual-stack IPv4/IPv6 binding
- **Local Development**: Falls back to IPv4-only (`127.0.0.1`) binding
- **Custom Host**: Use `SLACK_MCP_HOST=""` to force dual-stack binding

#### IPv6 Address Handling

The server properly formats IPv6 addresses in logs and handles both IPv4 and IPv6 client connections:

```
INFO SSE server starting with dual-stack IPv4/IPv6 binding
  port=8080 bind_address=:8080 railway_deployment=true
```

#### IPv6 Troubleshooting

**Problem**: IPv6 connections failing
**Solution**: 
1. Verify Railway supports IPv6 in your region
2. Check client IPv6 connectivity
3. Review server logs for binding errors

**Problem**: Server only binding to IPv4
**Solution**:
1. Set `SLACK_MCP_HOST=""` explicitly
2. Check Railway environment variables are set
3. Verify no IPv6 network restrictions

### Health Check Endpoints

Railway deployment includes comprehensive health monitoring:

#### Available Endpoints

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `/health` | Basic health status | JSON health summary |
| `/health/ready` | Readiness check | Slack API connectivity |
| `/health/live` | Liveness check | Application responsiveness |

#### Health Response Format

```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "version": "v1.0.0",
  "checks": {
    "slack_api": "ok",
    "cache": "ok"
  },
  "uptime": "1h30m45s"
}
```

#### Health Check Configuration

Railway automatically uses `/health` for monitoring. Configure health check behavior:

```bash
# Disable health checks (not recommended for Railway)
SLACK_MCP_HEALTH_ENABLED=false

# Custom health check timeout (Railway default: 30s)
# Set in railway.toml under [deploy] section
```

### Security Considerations

#### CORS Configuration

Configure allowed origins for browser-based MCP clients:

```bash
# Allow all origins (development only)
SLACK_MCP_CORS_ORIGINS="*"

# Allow specific origins (recommended for production)
SLACK_MCP_CORS_ORIGINS="https://claude.ai,https://cursor.com"

# Allow multiple origins
SLACK_MCP_CORS_ORIGINS="https://app1.com,https://app2.com,https://localhost:3000"
```

#### Rate Limiting

Protect against abuse with per-IP rate limiting:

```bash
# 60 requests per minute (default)
SLACK_MCP_RATE_LIMIT=60

# 120 requests per minute (higher limit)
SLACK_MCP_RATE_LIMIT=120

# 30 requests per minute (stricter limit)
SLACK_MCP_RATE_LIMIT=30
```

#### API Key Security

Secure your SSE endpoint with a strong API key:

```bash
# Generate a secure random key
openssl rand -base64 32

# Set as environment variable
SLACK_MCP_SSE_API_KEY="your-generated-secure-key"
```

### Deployment Best Practices

#### Environment Variable Management

1. **Use Railway's Environment Variables UI** for sensitive data
2. **Group variables by environment** (production, staging, development)
3. **Use strong API keys** for `SLACK_MCP_SSE_API_KEY`
4. **Rotate tokens regularly** for security

#### Monitoring and Logging

1. **Enable structured logging** with `SLACK_MCP_LOG_FORMAT=json`
2. **Set appropriate log level** (`info` for production, `debug` for troubleshooting)
3. **Monitor health endpoints** for service reliability
4. **Set up Railway alerts** for health check failures

#### Performance Optimization

1. **Configure appropriate rate limits** based on expected usage
2. **Use caching** for user and channel data (enabled by default)
3. **Monitor memory usage** through Railway metrics
4. **Scale horizontally** if needed using Railway's scaling features

### Troubleshooting

#### Common Issues

**Issue**: Server fails to start on Railway
**Solution**:
1. Check Railway logs for startup errors
2. Verify all required environment variables are set
3. Ensure Slack tokens are valid and not expired

**Issue**: Health checks failing
**Solution**:
1. Verify Slack API connectivity from Railway
2. Check network restrictions or firewall rules
3. Review health endpoint logs for specific errors

**Issue**: CORS errors in browser
**Solution**:
1. Add your domain to `SLACK_MCP_CORS_ORIGINS`
2. Verify the origin header in browser requests
3. Check for mixed HTTP/HTTPS content issues

**Issue**: Rate limiting too aggressive
**Solution**:
1. Increase `SLACK_MCP_RATE_LIMIT` value
2. Implement client-side request throttling
3. Consider using multiple API keys for different clients

#### Debug Mode

Enable debug logging for troubleshooting:

```bash
SLACK_MCP_LOG_LEVEL=debug
```

This provides detailed information about:
- Network binding and IPv6 configuration
- Slack API requests and responses
- Security middleware operations
- Health check execution details

#### Railway-Specific Debugging

1. **Check Railway Logs**: Use Railway dashboard to view application logs
2. **Verify Environment**: Ensure `RAILWAY_ENVIRONMENT` is set correctly
3. **Network Connectivity**: Test health endpoints from external tools
4. **Resource Usage**: Monitor CPU and memory usage in Railway metrics

### Migration from Local to Railway

#### Step 1: Prepare Configuration

1. Copy your local `.env` file settings
2. Add Railway-specific variables
3. Update CORS origins for your domain
4. Generate a secure SSE API key

#### Step 2: Update MCP Client Configuration

Change from local stdio transport:
```json
{
  "command": "npx",
  "args": ["-y", "slack-mcp-server@latest", "--transport", "stdio"]
}
```

To remote SSE transport:
```json
{
  "command": "npx",
  "args": [
    "-y", "mcp-remote",
    "https://your-app.railway.app/sse",
    "--header", "Authorization: Bearer ${SLACK_MCP_SSE_API_KEY}"
  ]
}
```

#### Step 3: Test and Validate

1. Deploy to Railway staging environment first
2. Test health endpoints: `https://your-app.railway.app/health`
3. Verify MCP client connectivity
4. Monitor logs for any issues
5. Promote to production when stable

### Advanced Configuration

#### Custom Domain Setup

1. Configure custom domain in Railway dashboard
2. Update `SLACK_MCP_BASE_URL` to match your domain
3. Update MCP client configuration with new URL
4. Ensure SSL certificate is properly configured

#### Multi-Environment Setup

```bash
# Production environment
RAILWAY_ENVIRONMENT=production
SLACK_MCP_LOG_LEVEL=info
SLACK_MCP_RATE_LIMIT=60

# Staging environment  
RAILWAY_ENVIRONMENT=staging
SLACK_MCP_LOG_LEVEL=debug
SLACK_MCP_RATE_LIMIT=120
```

#### Load Balancing Considerations

For high-traffic deployments:
1. Use Railway's horizontal scaling features
2. Implement session affinity if needed
3. Consider external load balancer for advanced routing
4. Monitor performance metrics and scale accordingly