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