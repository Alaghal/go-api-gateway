# go-api-gateway

High-performance API Gateway written in Go with routing, rate limiting, and observability.

## Features

- Lightweight HTTP server in Go
- Chi-based routing
- Health endpoint
- Environment-based configuration
- Graceful shutdown

## Run locally

```bash
go run ./cmd/server
```

## Routing and Middleware

The gateway uses `chi` for HTTP routing and includes:

- request timeout middleware
- request ID propagation via `X-Request-Id`
- request logging with status code and duration
- panic recovery middleware

### Available endpoints

- `GET /health`
- `GET /api/v1/ping`
## Rate Limiting

The gateway includes per-IP rate limiting middleware to protect upstream services from excessive traffic.

### Features

- token-bucket rate limiting
- configurable requests per second
- configurable burst capacity
- `429 Too Many Requests` response on limit exceeded
- `Retry-After` header
- logging for rate-limited requests

### Configuration

Environment variables:

- `RATE_LIMIT_RPS` - allowed requests per second per client IP
- `RATE_LIMIT_BURST` - burst capacity per client IP

Example:

```bash
export RATE_LIMIT_RPS=10
export RATE_LIMIT_BURST=20