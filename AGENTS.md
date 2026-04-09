# AGENTS.md

Guidance for AI coding agents working on MikMongo.

## Project Overview

MikMongo is a full-stack ISP management platform for Mikrotik routers. The Go backend handles customer management, subscription billing, payment processing (Xendit), router provisioning via the RouterOS API, WhatsApp notifications, and hotspot voucher sales. The React frontend provides admin, customer portal, and agent portal interfaces. The module name is `mikmongo` (see `go.mod`).

## Architecture Summary

```
Request flow:
  HTTP -> Gin Router -> Middleware (Auth/RBAC/CORS) -> Handler -> Service -> Domain -> Repository -> Postgres
                                                                             -> Mikrotik Service -> RouterOS API
                                                                             -> Queue Producer -> RabbitMQ
  Scheduler -> Service (cron jobs)
  Queue Consumer <- RabbitMQ -> Service
```

Layers are wired bottom-up in `cmd/server/main.go`:
1. **Config** (`internal/config/config.go`) loads `.env`
2. **Repository** (`internal/repository/`) defines interfaces; `internal/repository/postgres/` implements them with GORM
3. **Domain** (`internal/domain/{entity}/`) holds business rules
4. **Service** (`internal/service/`) orchestrates domain + repo + external calls
5. **Handler** (`internal/handler/`) maps HTTP to service calls via Gin
6. **Router** (`internal/router/`) registers Gin routes grouped by auth tier

Each layer has a `Registry` struct that collects all instances. Registries are wired together in `main.go`.

Supporting packages in `pkg/`:
- `jwt/` token generation and validation
- `response/` standardized JSON success/error responses
- `pagination/` cursor-based pagination helpers
- `rabbitmq/` AMQP client wrapper
- `redis/` caching
- `payment/xendit/` Xendit payment gateway
- `mikrotik/` RouterOS client, domain entities, and per-feature modules (ppp, hotspot, etc.)
- `gowa/` GoWA WhatsApp API client
- `ws/` WebSocket hub
- `logger/` zap-based logging
- `validator/` request validation

## Code Conventions

### Naming

| What | Convention | Example |
|---|---|---|
| Exported Go symbols | PascalCase | `CustomerService`, `ListCustomers` |
| Unexported Go symbols | camelCase | `parseFilter`, `repoRegistry` |
| DB columns | snake_case | `created_at`, `router_id` |
| Model files | `{entity}.go` | `internal/model/customer.go` |
| Repository interface | `{entity}_repo.go` | `internal/repository/customer_repo.go` |
| Repository impl | `{entity}_repo.go` | `internal/repository/postgres/customer_repo.go` |
| Service files | `{entity}_service.go` | `internal/service/customer_service.go` |
| Handler files | `{entity}_handler.go` | `internal/handler/customer_handler.go` |
| Domain files | `{entity}/domain.go` | `internal/domain/billing/domain.go` |
| Migration files | `{NNNN}_{table}.go` | `internal/migration/0001_customers.go` |
| API paths | plural nouns, kebab-case | `/customers`, `/bandwidth-profiles` |

### Patterns

- **Registry struct per layer**: Each layer directory has a `registry.go` with a `Registry` struct and `NewRegistry()` constructor. Add new fields there when adding entities.
- **Interface segregation**: Repository interfaces live in `internal/repository/`. Only the Postgres implementations in `internal/repository/postgres/` know about GORM.
- **Constructor injection**: Dependencies are injected via `NewXxxService(repo, domain, ...)` functions. No global state.
- **Setter injection for cross-cuts**: Some service-to-service dependencies use `SetXxxService()` to avoid import cycles (check `internal/service/registry.go`).
- **Soft deletes**: Every model embeds `gorm.DeletedAt`. Use GORM's `Unscoped()` when you need hard-delete behavior.
- **UUID primary keys**: Models use `uuid.UUID` (not auto-increment `uint`). The scaffolded `make model` target generates `uint` IDs, so change them to `uuid.UUID` for production models.
- **Encrypted secrets**: Mikrotik router passwords are AES-encrypted at rest. Use the encryption key passed from `cfg.JWT.Secret`.
- **Context-first**: All service and repository methods take `context.Context` as the first parameter.
- **Error wrapping**: Always use `fmt.Errorf("description: %w", err)`.

### Auth tiers

Three separate JWT auth flows, each with its own middleware:
- **Admin**: Full access, Casbin RBAC policies in `casbin_rules` table
- **Customer portal**: Self-service, restricted to own data
- **Agent portal**: Sales agents with limited scope

Middleware chain: `gin.Recovery` -> CORS -> Logger -> RequestID -> (Auth -> RBAC for admin routes).

### Standardized responses

All handlers use `pkg/response/`:
```go
response.Success(c, http.StatusOK, "message", data)
response.Error(c, http.StatusBadRequest, "message", err)
```

## How to Add a Feature (example: "product")

Follow these steps in order. Each `make` command scaffolds a file, but you must wire it manually.

```bash
# 1. Create model struct with Input/Filter
make model VAL=product
# Edit internal/model/product.go to add fields

# 2. Create repository interface + Postgres implementation
make repository VAL=product
# Implement CRUD methods in internal/repository/postgres/product_repo.go

# 3. Create domain logic
make domain VAL=product
# Add business rules in internal/domain/product/domain.go

# 4. Create service
make service VAL=product
# Wire repo dependency, add business methods in internal/service/product_service.go

# 5. Create handler with CRUD endpoints
make handler VAL=product
# Implement List/Get/Create/Update/Delete in internal/handler/product_handler.go

# 6. Wire everything in main.go
# Add to repoRegistry:    ProductRepo: pgRepo.ProductRepo
# Add to domainRegistry:  (if using domain)
# Add to serviceRegistry: (check service.NewRegistry signature)
# Add to handlerRegistry: (check handler.NewRegistry)

# 7. Create database migration
make migration VAL=products
# Add columns to the CREATE TABLE in internal/migration/NNNN_products.go

# 8. Register routes in internal/router/admin.go
# Follow the pattern: products := v1.Group("/products") { ... }

# 9. Add Casbin policy in internal/seeder/ if route needs auth

# 10. Verify
make test
```

### Adding a Mikrotik feature (example: "ppp")

```bash
# 1. Create RouterOS entity + interface
make mikrotik-domain VAL=ppp
# Edit pkg/mikrotik/domain/ppp.go with RouterOS fields and ros tags

# 2. Create service + repository + test
make mikrotik-module VAL=ppp
# Implement pkg/mikrotik/ppp/repository.go (RouterOS API calls)
# Write tests in pkg/mikrotik/ppp/ppp_test.go
# Wire into pkg/mikrotik/mikrotik.go facade

# 3. Create internal service that uses the mikrotik module
# Add to internal/service/mikrotik/ if it needs DB persistence
# Wire into service registry and handler
```

### Adding async processing (example: "suspend")

```bash
make queue-producer VAL=suspend   # creates internal/queue/producer/suspend_producer.go
make queue-consumer VAL=suspend   # creates internal/queue/consumer/suspend_consumer.go
# Wire consumer handler in cmd/server/main.go (see existing SetHandler pattern)
# Register goroutine in consumer/registry.go StartAll()
```

## Common Pitfalls

- **Forgetting to wire registries**: After any `make` scaffold, you must add the new field to the corresponding `registry.go` and initialize it in `NewRegistry()` or `main.go`. The make output prints reminders.
- **Migration blank import**: `cmd/server/main.go` must have `_ "mikmongo/internal/migration"`. Without it, Goose migrations never register their `init()` functions.
- **Import cycles**: The `internal/service/mikrotik` package depends on router services. If you need a mikrotik service from a non-mikrotik service, use callback functions (see `SetHandler` pattern in `main.go`) instead of direct imports.
- **UUID vs uint ID**: The scaffold generates `uint` IDs. The real models use `uuid.UUID`. Check existing models before copying the scaffold.
- **Running migrations**: After changing a model, you need a new migration file. Running the app with `AUTO_MIGRATE=true` resets the entire database. Never use this in production.
- **Registry field order**: The `NewRegistry()` constructors often take specific parameter order. Check the existing signature before adding a new argument.
- **Router nested groups**: Routes like `/routers/:router_id/ppp` use nested Gin groups. Follow the pattern in `internal/router/admin.go`.

## Key File Map

Files you will edit most often:

| File | Purpose |
|---|---|
| `cmd/server/main.go` | Dependency wiring. The single place everything connects. |
| `internal/config/config.go` | Environment variable loading. Add new config fields here. |
| `internal/router/admin.go` | Admin API route registration. |
| `internal/router/public.go` | Public routes (customer portal, webhooks). |
| `internal/router/agent_portal.go` | Agent portal routes. |
| `internal/service/registry.go` | Service registry. Add new service fields. |
| `internal/handler/registry.go` | Handler registry. Add new handler fields. |
| `internal/repository/registry.go` | Repository interface registry. |
| `internal/repository/postgres/registry.go` | Postgres implementation registry. Wire new repos here. |
| `internal/domain/registry.go` | Domain registry. |
| `internal/model/*.go` | Data models. One file per entity. |
| `internal/middleware/*.go` | Auth, CORS, RBAC middleware. |
| `internal/seeder/*.go` | Database seed data including Casbin rules. |
| `internal/migration/*.go` | Goose Go-based migrations. |
| `pkg/response/*.go` | HTTP response helpers used by all handlers. |
| `pkg/mikrotik/domain/*.go` | RouterOS entity definitions with `ros` struct tags. |
| `Makefile` | Code generation targets. |

## Testing Guidelines

```bash
# Unit tests (fast, no external dependencies)
make test
# Or directly:
go test -v -race ./internal/... ./pkg/...

# Integration tests (requires Postgres, Redis, RabbitMQ)
make test-integration
# Or:
go test -v -tags=integration ./tests/integration/...

# Generate mocks from go:generate directives
make generate-mocks
# Mocks land in tests/mocks/repository/ and tests/mocks/service/

# Coverage report
make test-coverage
# Opens coverage.html in browser

# Frontend tests
cd web && pnpm test

# Lint
make lint
```

When writing tests:
- Use table-driven test patterns
- Mock repository interfaces with generated mocks from `tests/mocks/`
- Test files live alongside the code they test (`*_test.go`)
- Use `t.Parallel()` where safe

## Make Targets Reference

| Target | Usage | Output |
|---|---|---|
| `make model VAL=x` | Scaffold model + Input + Filter | `internal/model/x.go` |
| `make repository VAL=x` | Scaffold repo interface + Postgres impl | `internal/repository/x_repo.go` + `internal/repository/postgres/x_repo.go` |
| `make domain VAL=x` | Scaffold domain logic | `internal/domain/x/domain.go` |
| `make service VAL=x` | Scaffold service | `internal/service/x_service.go` |
| `make handler VAL=x` | Scaffold CRUD handler | `internal/handler/x_handler.go` |
| `make migration VAL=x` | Scaffold Goose migration | `internal/migration/NNNN_x.go` |
| `make scheduler VAL=x` | Scaffold cron scheduler | `internal/scheduler/x_scheduler.go` |
| `make queue-producer VAL=x` | Scaffold RabbitMQ producer | `internal/queue/producer/x_producer.go` |
| `make queue-consumer VAL=x` | Scaffold RabbitMQ consumer | `internal/queue/consumer/x_consumer.go` |
| `make mikrotik-domain VAL=x` | Scaffold RouterOS entity + interface | `pkg/mikrotik/domain/x.go` |
| `make mikrotik-module VAL=x` | Scaffold RouterOS service + repo + test | `pkg/mikrotik/x/` |
| `make migrate-up` | Run pending migrations | Applies to DB |
| `make migrate-down` | Rollback last migration | Applies to DB |
| `make migrate-reset` | Rollback ALL migrations | Drops all tables |
| `make migrate-status` | Show migration status | Prints to stdout |
| `make seed` | Run migrations + seed data | Populates DB |
| `make fresh` | Reset + seed | Clean slate |
| `make generate-mocks` | Generate mock implementations | `tests/mocks/` |
| `make build` | Build Docker image | Docker image |
| `make http` | Run in Docker (HTTP mode) | Starts server |
| `make docker-up` | Start docker-compose infra | Postgres, Redis, RabbitMQ |
| `make docker-down` | Stop docker-compose | Stops containers |
| `make tidy` | Run `go mod tidy` | Cleans go.mod/go.sum |
| `make lint` | Run golangci-lint | Prints violations |
| `make test` | Run unit tests with race detector | Test results |
| `make test-coverage` | Tests + HTML coverage report | `coverage.html` |
| `make test-integration` | Run integration tests | Test results |
| `make dev` | Hot-reload with Air | Starts server |

## Frontend (web/)

Based on the shadcn-admin template. Relevant when editing full-stack features.

| Path | What lives there |
|---|---|
| `web/src/routes/` | TanStack Router file-based routes |
| `web/src/api/` | TanStack React Query data fetching hooks |
| `web/src/stores/` | Zustand state stores |
| `web/src/components/ui/` | Shadcn UI primitives |
| `web/src/features/` | Feature modules (one per domain entity) |
| `web/src/hooks/` | Shared custom hooks |

Stack: React 19, TypeScript, Vite 8, TanStack Router, TanStack React Query, Zustand, Shadcn UI, Tailwind CSS.
