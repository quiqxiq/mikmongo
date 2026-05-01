# MikMongo

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-Proprietary-red.svg)]()

MikMongo is a full-stack ISP management platform built around MikroTik routers. It handles customer lifecycle, PPPoE and Hotspot subscriptions, billing and invoicing, payment collection through Xendit, real-time router monitoring via InfluxDB, sales agent portals, and WhatsApp notification delivery.

## Features

- **Customer Management** -- registration, profiles, portal access, audit logs
- **Subscription Engine** -- PPPoE and Hotspot plans with automated provisioning on MikroTik routers
- **Billing & Invoicing** -- invoice generation, line items, payment allocation, sequence counters
- **Payment Gateway** -- Xendit integration with webhook handling for payment confirmation
- **MikroTik Router Control** -- bandwidth profiles, router device management, RouterOS API integration
- **Real-Time Monitoring** -- InfluxDB-backed metrics collection and dashboard charts
- **Sales Agent Portal** -- agent-specific invoice management, commission tracking
- **WhatsApp Notifications** -- GoWA gateway integration for customer and agent messaging
- **RBAC Authorization** -- Casbin-powered role-based access control with JWT authentication
- **Background Processing** -- RabbitMQ producers/consumers for async tasks, cron-based schedulers
- **Cash Management** -- petty cash funds, cash entries, financial reporting

## Tech Stack

### Backend

| Component | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Framework | Gin |
| ORM | GORM |
| Database | PostgreSQL 17 |
| Cache | Redis 7 |
| Message Queue | RabbitMQ |
| Time-Series DB | InfluxDB 3 Core |
| Auth | JWT + Casbin RBAC |
| Migrations | Goose |
| Logging | Zap |
| Payments | Xendit Go SDK |
| Router API | go-routeros (RouterOS API client) |

### Frontend

The web UI is maintained in a separate repository. This repository contains backend services and shared backend packages only.

### Infrastructure

| Component | Technology |
|---|---|
| Containers | Docker + docker-compose |
| Reverse Proxy | Nginx |
| Monitoring Stack | docker-compose.monitor.yml |

## Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Goose CLI (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- golangci-lint (optional, for linting)
- A MikroTik router accessible via API (port 8728)

## Quick Start

```bash
# 1. Clone the repository
git clone <repo-url> && cd mikmongo

# 2. Configure environment
cp .env.example .env
# Edit .env with your database, Redis, RabbitMQ, MikroTik, InfluxDB, and Xendit credentials

# 3. Start infrastructure services
docker-compose -f deployments/docker-compose.yml up -d

# 4. Run database migrations
make migrate-up

# 5. Seed initial data (roles, permissions, admin user)
make seed

# 6. Start the backend server
go run cmd/server/main.go

```

The API runs on `http://localhost:8080`.

## Environment Setup

Copy `.env.example` to `.env` and configure the following sections:

```bash
cp .env.example .env
```

### Required Configuration

| Variable Group | Key Variables | Description |
|---|---|---|
| Application | `APP_PORT`, `APP_ENV` | Server port and environment mode |
| Database | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | PostgreSQL connection |
| Redis | `REDIS_HOST`, `REDIS_PORT` | Cache and session store |
| RabbitMQ | `RABBITMQ_HOST`, `RABBITMQ_USER`, `RABBITMQ_PASSWORD` | Message broker |
| JWT | `JWT_SECRET`, `JWT_EXPIRY` | Authentication tokens |
| MikroTik | `MIKROTIK_HOST`, `MIKROTIK_PORT`, `MIKROTIK_USER`, `MIKROTIK_PASSWORD` | RouterOS API access |
| Xendit | `XENDIT_SECRET_KEY`, `XENDIT_PUBLIC_KEY`, `XENDIT_WEBHOOK_SECRET` | Payment gateway |
| WhatsApp | `GOWA_BASE_URL`, `GOWA_DEVICE_ID` | Notification gateway |

### DSN Format

The `DB_DSN` variable must be a valid PostgreSQL connection string:

```
DB_DSN=postgres://mikmongo:mikmongo@localhost:5432/mikmongo?sslmode=disable
```

### Docker Services Defaults

The `docker-compose.yml` creates services with these defaults:

| Service | Credentials | Port |
|---|---|---|
| PostgreSQL | `mikmongo` / `mikmongo` | 5432 |
| Redis | no password | 6379 |
| RabbitMQ | `mikmongo` / `mikmongo` | 5672 (AMQP), 15672 (Management UI) |

These match the defaults in `.env.example`, so local development works without changes after `docker-compose up`.

## Project Structure

```
mikmongo/
├── cmd/                        # Application entry points
│   ├── server/                 # Main API server
│   ├── seed/                   # Database seeder
│   ├── migrate/                # Standalone migration runner
│   ├── mikrotik/               # MikroTik CLI tools
│   └── full_test/              # Full stack test runner
├── internal/                   # Private application code
│   ├── casbin/                 # RBAC policy definitions
│   ├── collector/              # Collector service orchestration
│   ├── config/                 # Configuration loading
│   ├── domain/                 # Domain logic modules
│   ├── dto/                    # Data transfer objects
│   │   └── mikrotik/           # MikroTik request/response DTOs
│   ├── handler/                # HTTP handlers (Gin controllers)
│   │   └── mikrotik/           # MikroTik-specific handlers
│   ├── middleware/             # HTTP middleware (auth, logging, CORS)
│   ├── migration/              # Goose Go-based migrations
│   ├── model/                  # GORM models / entities
│   ├── notification/           # WhatsApp notification dispatch
│   ├── queue/                  # RabbitMQ producers and consumers
│   ├── repository/             # Repository interfaces + GORM implementations
│   │   └── postgres/           # PostgreSQL-specific implementations
│   ├── router/                 # Route registration
│   │   └── mikrotik/           # MikroTik API routes
│   ├── scheduler/              # Cron job schedulers
│   ├── seeder/                 # Seed data definitions
│   └── service/                # Business logic layer
│       ├── mikrotik/           # MikroTik service adapters
│       └── mocks/              # Generated test mocks
├── pkg/                        # Shared public packages
│   ├── gowa/                   # GoWA WhatsApp client
│   ├── jwt/                    # JWT token utilities
│   ├── logger/                 # Zap logger setup
│   ├── mikrotik/               # MikroTik RouterOS SDK
│   │   ├── collector/          # Tier 1/2/3 collector pipeline
│   │   └── domain/             # RouterOS entity definitions
│   ├── pagination/             # Cursor/offset pagination
│   ├── payment/                # Xendit payment integration
│   ├── rabbitmq/               # RabbitMQ client wrapper
│   ├── redis/                  # Redis client wrapper
│   ├── response/               # Standardized HTTP responses
│   ├── validator/              # Custom validation rules
│   └── ws/                     # WebSocket utilities
├── tests/                      # Integration and HTTP tests
│   ├── http/                   # HTTP handler tests
│   ├── integration/            # Integration tests (build tag: integration)
│   └── mocks/                  # Generated mocks
├── deployments/                # Infrastructure configuration
│   ├── docker-compose.yml      # Development services
│   ├── docker-compose.monitor.yml # Monitoring stack (InfluxDB, Grafana)
│   ├── mikrotik-test/          # RouterOS test container
│   ├── Dockerfile              # Multi-stage Go build
│   └── nginx.conf              # Reverse proxy config
├── docs/                       # Documentation
│   ├── openapi.docs.yml        # OpenAPI 3 specification
│   ├── COLLECTOR_V2_ARCHITECTURE.md
│   └── COLLECTOR_IMPLEMENTATION_GUIDE.md
├── utils/                      # Shared utilities (encryption, etc.)
├── Makefile                    # Build and code generation targets
├── go.mod
└── .env.example                # Environment template
```

## Available Make Commands

### Infrastructure

| Command | Description |
|---|---|
| `make docker-build` | Build the Docker image from `deployments/Dockerfile` |
| `make docker-up` | Start PostgreSQL, Redis, RabbitMQ containers |
| `make docker-down` | Stop and remove all containers |

### Database

| Command | Description |
|---|---|
| `make migrate-up` | Run all pending Goose migrations |
| `make migrate-down` | Roll back the last migration |
| `make migrate-reset` | Roll back all migrations (destructive) |
| `make migrate-status` | Show current migration status |
| `make seed` | Run migrations and seed initial data |
| `make fresh` | Reset all migrations, then seed from scratch |

### Code Generation

All generators create a file plus update the corresponding `registry.go`. Replace `<name>` with your resource name.

| Command | Output |
|---|---|
| `make model VAL=<name>` | `internal/model/<name>.go` |
| `make domain VAL=<name>` | `internal/domain/<name>/domain.go` |
| `make service VAL=<name>` | `internal/service/<name>_service.go` |
| `make handler VAL=<name>` | `internal/handler/<name>_handler.go` |
| `make repository VAL=<name>` | Interface + GORM implementation in `internal/repository/` |
| `make migration VAL=<name>` | `internal/migration/XXXX_<name>.go` |
| `make scheduler VAL=<name>` | `internal/scheduler/<name>_scheduler.go` |
| `make queue-producer VAL=<name>` | `internal/queue/producer/<name>_producer.go` |
| `make queue-consumer VAL=<name>` | `internal/queue/consumer/<name>_consumer.go` |
| `make mikrotik-domain VAL=<name>` | `pkg/mikrotik/domain/<name>.go` |
| `make mikrotik-module VAL=<name>` | `pkg/mikrotik/<name>/` (service + repo + test) |

### Testing & Quality

| Command | Description |
|---|---|
| `make test` | Run unit tests with race detector |
| `make test-integration` | Run integration tests |
| `make test-coverage` | Generate HTML coverage report |
| `make lint` | Run golangci-lint |
| `make generate-mocks` | Regenerate test mocks from `go:generate` directives |
| `make tidy` | Run `go mod tidy` |

## API Documentation

An OpenAPI specification is available at `docs/openapi.docs.yml`. Import it into Swagger UI, Redoc, or any OpenAPI-compatible tool to explore all endpoints.

### API Route Groups

The backend exposes four route groups, each with its own auth scope:

| Group | Prefix | Auth | Description |
|---|---|---|---|
| Public | `/api/v1/auth/*`, `/api/v1/register`, `/api/v1/webhooks/*`, `/health` | None / Webhook signature | Login, registration, payment webhooks |
| Admin API | `/api/v1/*` | JWT + Casbin RBAC (`mw.Auth.Authenticate` → `mw.RBAC`) | Full admin/operator dashboard API |
| Customer Portal | `/portal/v1/*` | JWT (`mw.PortalAuth.AuthenticatePortal`) | Customer self-service (profile, invoices, payments) |
| Agent Portal | `/agent-portal/v1/*` | JWT (`mw.AgentPortalAuth.AuthenticateAgentPortal`) | Sales agent self-service |

### Key Endpoints

| Area | Endpoints |
|---|---|
| Auth | `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me` |
| Users | CRUD `/api/v1/users` |
| Customers | CRUD `/api/v1/customers`, `POST /:id/activate-account`, `POST /:id/deactivate-account` |
| Routers | CRUD `/api/v1/routers`, `POST /:id/sync`, `POST /:id/test-connection`, `POST /routers/sync-all` |
| Subscriptions | CRUD `/api/v1/routers/:router_id/subscriptions`, activate/suspend/isolate/restore/terminate |
| Bandwidth Profiles | CRUD `/api/v1/routers/:router_id/bandwidth-profiles` |
| Invoices | `GET /api/v1/invoices`, `GET /overdue`, `POST /trigger-monthly` |
| Payments | CRUD `/api/v1/payments`, `POST /:id/confirm`, `POST /:id/reject`, `POST /:id/refund`, `POST /:id/initiate-gateway` |
| Registrations | `GET /api/v1/registrations`, `POST /:id/approve`, `POST /:id/reject` |
| Sales Agents | CRUD `/api/v1/sales-agents`, profile-prices, per-agent invoices |
| Agent Invoices | CRUD `/api/v1/agent-invoices`, `PUT /:id/pay`, `PUT /:id/cancel`, `POST /process` |
| Hotspot Sales | `GET /api/v1/routers/:router_id/hotspot-sales` |
| Cash Management | CRUD `/api/v1/cash-entries`, approve/reject; CRUD `/api/v1/petty-cash`, topup |
| Reports | `GET /api/v1/reports/{summary,subscriptions,cash-flow,cash-balance,reconciliation}` |
| Settings | `GET/PUT /api/v1/settings` |
| MikroTik PPP | `/api/v1/routers/:router_id/ppp/{profiles,secrets,active}` (+ WebSocket streams) |
| MikroTik Hotspot | `/api/v1/routers/:router_id/hotspot/{profiles,users,active,hosts,servers}` (+ WS) |
| MikroTik Network | `/api/v1/routers/:router_id/{queue,firewall,ip}/*` |
| MikroTik Monitor | `/api/v1/routers/:router_id/monitor/{system-resource,interfaces}` (+ WS traffic/logs/ping) |
| MikroTik Mikhmon | `/api/v1/routers/:router_id/mikhmon/{vouchers,profiles,reports,expire}` |
| **Collector (cached reads)** | `GET /api/v1/routers/:router_id/cached/{ppp-active,hotspot-active,ppp-secrets,queue-stats}`, `GET /logs`, `GET /collector/status` |
| MikroTik Raw | `POST /api/v1/routers/:router_id/raw/run`, WS `/raw/ws/listen` |
| Webhooks | `POST /api/v1/webhooks/xendit`, `POST /api/v1/webhooks/midtrans` |
| Customer Portal | `/portal/v1/{login,profile,subscriptions,invoices,payments}` |
| Agent Portal | `/agent-portal/v1/{login,profile,invoices,sales}` |

## Data Collection Architecture

MikMongo separates live RouterOS API calls from cached reads using a tiered collector pipeline. This reduces load on MikroTik devices and provides sub-second response times for dashboard queries.

```
RouterOS device ──(poll)──> Collector (Tier 1/2/3) ──> Redis (streams + hashes) ──> /cached/* HTTP handlers
                                                  ╰──> InfluxDB 3 Core (time-series metrics)
```

- **Tier 1** (`pkg/mikrotik/collector/pipeline/time_series`) — high-frequency metrics written to InfluxDB (bandwidth, CPU, memory)
- **Tier 2** (`pkg/mikrotik/collector/pipeline/operational/tier2_collector.go`) — operational snapshots (PPP active, hotspot active) cached in Redis hashes
- **Tier 3** (`pkg/mikrotik/collector/pipeline/operational/tier3_collector.go`) — low-frequency config (PPP secrets, queue configs) cached in Redis

Handlers under `/api/v1/routers/:router_id/cached/*` read directly from Redis, never touching the router. Router logs are tailed into a Redis Stream and exposed via `GET /logs`. The collector status endpoint reports pipeline health, last sync time, and error counts.

See `docs/COLLECTOR_V2_ARCHITECTURE.md` and `docs/COLLECTOR_IMPLEMENTATION_GUIDE.md` for the full design and operational runbook.

## Development Guide

### Architecture

The project follows a layered architecture with strict dependency direction:

```
Handler (HTTP) -> Service (business logic) -> Repository (data access) -> Database
```

Each layer has its own registry pattern for dependency injection. When you generate a new module with `make domain/service/handler/repository`, the corresponding `registry.go` gets updated automatically with the new field and factory function. You still need to wire the constructor call in the registry's `New*()` function.

### Adding a New Module

To add a new feature module (e.g., "tax"):

```bash
make model VAL=tax            # 1. Create the GORM model
make repository VAL=tax       # 2. Create interface + GORM implementation
make service VAL=tax          # 3. Create the service layer
make handler VAL=tax          # 4. Create HTTP handlers
make migration VAL=taxes      # 5. Create the database migration
```

Then wire everything together:

1. Edit `internal/repository/postgres/registry.go` to initialize the new repo
2. Edit `internal/service/registry.go` to inject the repo into the service
3. Edit `internal/handler/registry.go` to inject the service into the handler
4. Register routes in `internal/router/`
5. Run `make migrate-up` to apply the new migration

### Adding a Background Job

```bash
make queue-producer VAL=tax_report    # Creates the event producer
make queue-consumer VAL=tax_report    # Creates the event consumer
```

Register the consumer goroutine in `internal/queue/consumer/registry.go` within `StartAll()`.

### Adding a Scheduled Task

```bash
make scheduler VAL=tax_report
```

Register the scheduler in `internal/scheduler/registry.go` within `NewRegistry()`.

### Adding a MikroTik Resource

```bash
make mikrotik-domain VAL=ppp     # Define the RouterOS entity
make mikrotik-module VAL=ppp     # Create service + repository + tests
```

Implement the RouterOS API calls in `pkg/mikrotik/ppp/repository.go`.

### Frontend Development

The frontend has been moved to a separate repository and is no longer included in this codebase.

## Testing

### Unit Tests

```bash
make test
# Equivalent to:
go test -v -race ./internal/... ./pkg/...
```

### Integration Tests

Integration tests require running infrastructure services. Start them first:

```bash
make docker-up
```

Then run with the `integration` build tag:

```bash
make test-integration
# Equivalent to:
go test -v -tags=integration ./tests/integration/...
```

### Test Configuration

Integration tests use separate environment variables (prefixed with `TEST_`):

```
TEST_DB_HOST=localhost
TEST_DB_NAME=mikmongo_test
TEST_REDIS_DB=15
TEST_MIKROTIK_HOST=192.168.27.1
```

### Coverage Report

```bash
make test-coverage
# Opens coverage.html in your browser
```

### Mock Generation

Mocks are generated from `go:generate` directives in repository interfaces:

```bash
make generate-mocks
```

## Deployment

### Docker Build

```bash
make docker-build
# Uses deployments/Dockerfile (multi-stage Go build)
```

### Production Docker Compose

The `docker-compose.yml` has a commented-out `app` service block. Uncomment and configure it for production deployment. The Nginx block is also available for reverse proxying the API.

### Manual Deployment

```bash
# Build the binary
CGO_ENABLED=0 go build -o mikmongo ./cmd/server

# Run with your .env
./mikmongo
```

### Ports Summary

| Service | Port | Purpose |
|---|---|---|
| API Server | 8080 | REST API |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Cache |
| RabbitMQ | 5672 | AMQP |
| RabbitMQ UI | 15672 | Management dashboard |
| InfluxDB | 8181 | Time-series metrics |
