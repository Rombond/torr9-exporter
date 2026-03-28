# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Architecture Overview

## High-Level Summary
This is a **Go-based Prometheus exporter** that scrapes user metrics from the Torr9 API and exposes them via the Prometheus HTTP interface. The application uses:
- Gin for HTTP routing
- Prometheus client library for metrics exposition
- OAuth2-like token-based authentication with Torr9 API

## Key Components

### 1. Authentication Layer (`main.go` lines 56-130)
- `Torr9Client` struct manages login tokens and HTTP sessions
- Stores JWT token after successful login at `/api/v1/auth/login`
- Uses mutex for thread-safe token access
- Auto-authenticates on startup if credentials are provided via environment variables

### 2. Metrics Collection (`main.go` lines 132-172, 186-221)
- `FetchMetrics()` calls `/api/v1/users/me` with the auth token
- Returns `UserMetrics` struct (upload/download bytes, jeton balance)
- Prometheus gauges registered to track these metrics over time

### 3. HTTP Server (`main.go` lines 227-251)
- Serves `/metrics` endpoint that scrapes and updates Prometheus gauges, then exposes them
- Health check at `/health` for monitoring tools
- Runs as Gin server on configurable port (default: 9090)

### 4. Configuration (via `.env`)
```
TORR9_API_BASE_URL    # Torr9 API host (e.g., torr9.net)
TORR9_USERNAME        # API credentials (required for auth)
TORR9_PASSWORD        # API credentials (required for auth)
PORT                  # Server port (default: 9090)
METRICS_PATH          # Metrics path (default: /metrics)
SCRAPE_INTERVAL       # How often to refresh metrics (configured in cron, not used here)
```

## Data Flow
1. Startup: Load config → attempt auto-login → start Gin server
2. `/` requests: Check auth → call Torr9 API → update Prometheus gauges → expose via promhttp.Handler()
3. External Prometheus server scrapes `/metrics` to collect data

# Commands

## Building and Running

```bash
# Build
go build -o torr9_exporter .

# Run with environment variables
TORR9_USERNAME=<user> TORR9_PASSWORD=<pass> go run .

# Run locally using docker-compose (requires Docker)
docker-compose up

# In development, rebuild after code changes
docker-compose up --build
```

## Common Tasks

### Run a single test
```bash
go test -v ./...  # Run all tests with verbose output
go test -run TestSpecificName ./...  # Run specific test by name
```

### View metrics locally
```bash
curl http://localhost:9090/metrics
```

### Health check
```bash
curl http://localhost:9090/health
```

## Docker Commands

```bash
# Build and run via docker-compose
docker-compose up --build

# Access logs
docker-compose logs -f

# Stop
docker-compose down
```

# Environment Variables Reference

Required for operation:
- `TORR9_USERNAME`: Torr9 API username
- `TORR9_PASSWORD`: Torr9 API password

Optional (with defaults):
- `TORR9_API_BASE_URL`: Defaults to "torr9.net"
- `PORT`: Server listen port, default 9090
- `METRICS_PATH`: Metrics endpoint path, default /metrics
- `SCRAPE_INTERVAL`: Not used by application (configured externally)

# Development Workflow

1. Configure `.env` with Torr9 credentials
2. Run via `go run .` for quick iteration
3. Or use `docker-compose up` for containerized deployment
4. Check Prometheus at the configured endpoint or set up a scraping job in your Prometheus config
