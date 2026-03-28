# Torr9 Exporter

A vibe-coded Go-based Prometheus exporter that scrapes user metrics from the Torr9 API and exposes them via the Prometheus HTTP interface.

It's entirely vibe coded—no elaborate build system, no complex tooling, just straight Go code running as a simple HTTP service that:
- Authenticates with Torr9 using your credentials
- Scrapes `/api/v1/users/me` for upload/download bytes and jeton balance
- Exposes everything at `/metrics` for Prometheus to scrape

## Overview

This exporter collects metrics such as:
- Upload bytes
- Download bytes
- Jeton balance

Metrics are exposed at a configurable endpoint and can be scraped by any Prometheus instance.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ Torr9 API   │────▶│ Exporter     │◀────│ Prometheus   │
│ (user data) │     │ (scrapes &   │     │ (collects)  │
└─────────────┘     │ exposes /    │     └─────────────┘
                    │ metrics      │
                    └──────────────┘
```

## Requirements

- Go 1.21+
- Docker (optional, for containerized deployment)

## Configuration

Create a `.env` file in the project root:

```bash
TORR9_API_BASE_URL=torr9.net
TORR9_USERNAME=<your_username>
TORR9_PASSWORD=<your_password>
PORT=9090
METRICS_PATH=/metrics
SCRAPE_INTERVAL=30s
```

| Variable | Description | Default |
|----------|-------------|---------|
| `TORR9_API_BASE_URL` | Torr9 API host | `torr9.net` |
| `TORR9_USERNAME` | API username | *required* |
| `TORR9_PASSWORD` | API password | *required* |
| `PORT` | Server listen port | `9090` |
| `METRICS_PATH` | Metrics endpoint path | `/metrics` |

## Quick Start

### Local Development

```bash
# Set environment variables
export TORR9_USERNAME=<your_username>
export TORR9_PASSWORD=<your_password>

# Build
go build -o torr9_exporter .

# Run
./torr9_exporter
```

Or run directly:

```bash
TORR9_USERNAME=<user> TORR9_PASSWORD=<pass> go run .
```

### Docker

```bash
docker-compose up --build
```

View logs:

```bash
docker-compose logs -f
```

Stop:

```bash
docker-compose down
```

## Endpoints

- `GET /` - Metrics endpoint (scrapes Torr9 API and exposes Prometheus metrics)
- `GET /health` - Health check endpoint

### Viewing Metrics Locally

After starting the exporter:

```bash
curl http://localhost:9090/metrics
```

Expected output:

```text
# HELP torr9_exporter_upload_bytes_total Total upload bytes
# TYPE torr9_exporter_upload_bytes_total gauge
torr9_exporter_upload_bytes_total 12345678
# HELP torr9_exporter_download_bytes_total Total download bytes
# TYPE torr9_exporter_download_bytes_total gauge
torr9_exporter_download_bytes_total 87654321
# HELP torr9_exporter_jetons_balance Current jeton balance
# TYPE torr9_exporter_jetons_balance gauge
torr9_exporter_jetons_balance 1500.5
```

## Prometheus Configuration

Add a scrape job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'torr9_exporter'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: /metrics
```

## Development Workflow

1. Configure `.env` with your Torr9 credentials
2. Run via `go run .` for quick iteration
3. Or use `docker-compose up` for containerized deployment
4. Check Prometheus at the configured endpoint

### Running Tests

```bash
go test -v ./...              # Run all tests
go test -run TestSpecificName ./...  # Run specific test
```

## License

[MIT](./LICENSE)
