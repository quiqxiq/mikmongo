# MikMongo -- Technical Analysis

> Full-stack ISP (Internet Service Provider) management platform for MikroTik routers.
> Auto-generated architectural analysis covering every layer of the codebase.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Backend -- Entry Points (`cmd/`)](#3-backend----entry-points-cmd)
4. [Backend -- Internal Layers (`internal/`)](#4-backend----internal-layers-internal)
5. [Backend -- Shared Libraries (`pkg/`)](#5-backend----shared-libraries-pkg)
6. [Frontend (`web/`)](#6-frontend-web)
7. [Database Layer](#7-database-layer)
8. [Infrastructure](#8-infrastructure)
9. [API Routes](#9-api-routes)
10. [Security](#10-security)
11. [Patterns and Conventions](#11-patterns-and-conventions)
12. [Strengths](#12-strengths)
13. [Weaknesses and Improvement Areas](#13-weaknesses-and-improvement-areas)
14. [Data Flow Diagrams](#14-data-flow-diagrams)

---

## 1. Overview

### 1.1 Project Identity

| Attribute | Value |
|-----------|-------|
| Module | `mikmongo` |
| Go Version | 1.25.5 |
| Primary Purpose | ISP management for MikroTik routers |
| Domain | Billing, subscriptions, customer management, network device control, hotspot voucher sales, payment processing |

### 1.2 Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Language (Backend) | Go | 1.25.5 |
| HTTP Framework | Gin | v1.12 |
| ORM | GORM | v1.31 |
| Database | PostgreSQL | 17 (Alpine) |
| Cache / Session Store | Redis | 7 (Alpine) |
| Message Queue | RabbitMQ | management-alpine |
| Time-Series DB | InfluxDB | 3 Core |
| Language (Frontend) | TypeScript | 6 |
| UI Framework | React | 19 |
| Build Tool | Vite | 8 (SWC plugin) |
| UI Components | Shadcn UI (new-york) + Radix UI + Tailwind CSS v4 | -- |
| Routing | TanStack Router (file-based, auto code-split) | -- |
| Data Fetching | TanStack Query v5 | -- |
| State Management | Zustand v5 (persist middleware) | -- |
| Validation (Backend) | go-playground/validator | v10 |
| Validation (Frontend) | Zod v4 | -- |
| Auth | golang-jwt/jwt (HS256) | v5 |
| RBAC | Casbin (GORM adapter) | v3 |
| Payment Gateway | Xendit Go SDK | v7 |
| WhatsApp Gateway | GoWA (go-whatsapp-web-multidevice) | -- |
| Migrations | Goose (Go-based SQL) | v3 |
| Logging | Zap | v1.27 |
| Charts | Recharts | v3 |
| Forms | react-hook-form v7 + @hookform/resolvers | -- |

### 1.3 Repository Layout

```
mikmongo-fully/
  cmd/              # 5 entry points (server, collector, mikrotik, seed, migrate)
  internal/         # Private application code (16 subdirectories)
  pkg/              # 12 shared library packages
  web/              # React frontend (src/ with features, api, hooks, stores, routes)
  deployments/      # Docker, Nginx, docker-compose files
  docs/             # OpenAPI spec, collector guides, RouterOS API schema
  tests/            # HTTP test collections, integration tests, mocks
  utils/            # Encryption utility
  Makefile          # 30+ code generation + dev targets (908 lines)
  go.mod / go.sum   # Go module definition
```

---

## 2. Architecture

### 2.1 High-Level Architecture

MikMongo uses a **layered architecture** with a **Registry pattern** for dependency injection. Each layer holds a registry struct that aggregates all instances of that layer, constructed in `cmd/server/main.go` with explicit bottom-up wiring.

```
                    +------------------+
                    |   HTTP Client    |  (React SPA / API consumers)
                    +--------+---------+
                             |
                    +--------v---------+
                    |   Nginx / Vite   |  (reverse proxy / dev server)
                    |   proxy -> :8080 |
                    +--------+---------+
                             |
        +--------------------v---------------------+
        |              Gin Router                   |
        |  (public, admin /api/v1, /portal,         |
        |   /agent-portal, websocket)               |
        +----+---------+---------+--------+---------+
             |         |         |        |
        +----v---+ +---v---+ +--v----+ +-v------+
        |Middleware| |Handler| |Service| |WebSocket|
        +----+---+ +---+---+ +--+----+ +--------+
             |         |         |
        +----v---------v---------v-----+
        |       Domain + Repository     |
        +----+---------+---------+------+
             |         |         |
        +----v---+ +---v----+ +--v------+
        |  GORM  | |  Redis | | RabbitMQ|
        +----+---+ +---+----+ +---------+
             |         |
        +----v---------v-----+
        | PostgreSQL | Redis  |
        +----+--------+-------+
             |
        +----v------------+
        |  MikroTik API   |  (pkg/mikrotik/)
        |  3-Pipeline      |
        |  Collector       |
        +--+-+--+----------+
           |  |  |
    +------+  |  +-------+
    |         |          |
+---v---+ +--v---+ +----v-----+
|InfluxDB| |Redis | |Direct API|
+--------+ +------+ +----------+
```

### 2.2 Backend Layer Dependency Flow

```
cmd/server/main.go
  |
  +-> config.Load()            # .env -> typed Config struct
  +-> logger.New(env)           # Zap wrapper
  +-> gorm.Open(postgres)       # Database
  +-> redis.NewClient()         # Cache
  +-> rabbitmq.NewClient()      # Queue
  +-> jwt.NewService()          # Token service
  |
  +-> repository layer          # 19 interfaces + postgres implementations
  +-> domain layer              # 7 pure-logic modules
  +-> service.NewRegistry()     # 13 services + mikrotik sub-services
  +-> handler layer             # 32 handler structs
  +-> casbin.NewEnforcer(db)    # RBAC
  +-> middleware.NewRegistry()  # 9 middlewares
  +-> router.New()              # 3 route groups
  +-> scheduler.NewRegistry()   # 6 cron jobs
  +-> server.Run()              # Graceful shutdown on SIGINT/SIGTERM
```

### 2.3 Frontend Architecture

```
web/src/main.tsx
  |
  +-> QueryClientProvider (TanStack Query)
  +-> RouterProvider (TanStack Router, file-based)
  +-> ThemeProvider (light/dark/system, cookie-persisted)
  +-> FontProvider (inter/manrope/system)
  +-> DirectionProvider (ltr/rtl)
  |
  +-> routes/
       +-> (auth)/              # Public auth pages (sign-in, sign-up, OTP)
       +-> (errors)/            # Error pages (401, 403, 404, 500, 503)
       +-> _authenticated/      # Guarded by adminIsAuthenticated check
       |    +-> Dashboard, Users, Customers, Routers, Subscriptions,
       |       Bandwidth Profiles, Invoices, Payments, Cash,
       |       Mikhmon (hotspot/vouchers/report), Reports, Settings
       +-> customer portal/    # Guarded by customerIsAuthenticated
       +-> agent portal/       # Guarded by agentIsAuthenticated
```

---

## 3. Backend -- Entry Points (`cmd/`)

### 3.1 `cmd/server/main.go` (297 lines)

The primary application server. Initialization order:

1. `config.Load()` -- reads `.env` via `godotenv`
2. `logger.New(env)` -- Zap production (JSON) or development (color)
3. `ws.SetAllowedOrigins()` -- WebSocket origin allowlist
4. `gorm.Open(postgres.Open(cfg.GetDSN()))` -- PostgreSQL via pgx driver
5. Auto-migrate (`AUTO_MIGRATE=true`) -- `goose.Reset` + `goose.Up` + `seeder.Run`
6. `redis.NewClient()` -- cache, session, rate-limiting
7. `rabbitmq.NewClient()` -- non-fatal on failure
8. `jwt.NewService()` -- HS256 with configurable expiry
9. 19 repositories via `repository.NewRegistry()`
10. 7 domain modules via `domain.NewRegistry()`
11. Queue registry with billing/suspend/notification producers+consumers
12. GoWA WhatsApp client + notification sender
13. Service registry (13 services + mikrotik sub-services)
14. Mikrotik registry injected into services
15. Queue consumers wired with service callbacks
16. Xendit payment provider
17. 32 handler structs
18. Casbin RBAC enforcer
19. Middleware registry (auth, RBAC, CORS, rate-limit, logger, request ID)
20. Gin router with all route groups
21. Scheduler registry (6 cron jobs)
22. `http.Server` with graceful shutdown (30s timeout)

### 3.2 `cmd/collector/main.go` (196 lines)

CLI test runner for the 4-tier collection system:
- Flags: `-host`, `-port`, `-user`, `-pass`, `-redis`, `-duration` (default 60s)
- Connects to MikroTik, wraps with `mikrotik.NewClientFromConnection`
- Creates `collector.Collector` with Redis backend
- Registers router with `DefaultISPSpecs()` (Tier 1+2) + `Tier3Specs()` (Tier 3)
- Prints pool stats every 10s, reads cached data for `interface:stats`, `ppp:active`, `ppp:secrets`
- Graceful shutdown on SIGINT/SIGTERM

### 3.3 `cmd/mikrotik/main.go` (491 lines)

Standalone MikroTik monitoring dashboard with InfluxDB v3:
- Streams 7 RouterOS commands to Redis Pub/Sub + InfluxDB v3:
  - `system-resource` (every 2s), `interface-traffic` (1s), `queue-stats` (1s)
  - `ip-addresses`, `ppp-active`, `dhcp-leases`, `router-log` (all follow mode)
- HTTP endpoints: `/api/live` (SSE), `/api/history` (InfluxDB SQL), `/api/interfaces`, `/api/queues`
- Serves embedded static dashboard from `cmd/mikrotik/static/index.html` (873 lines, Chart.js, dark theme)

### 3.4 `cmd/seed/main.go` (63 lines)

Database seeder: loads `.env`, connects via pgx, runs `goose.Reset` then `goose.Up`, then `seeder.New(db, cfg).Run(ctx)`.

### 3.5 `cmd/migrate/main.go` (36 lines)

Standalone migration runner: reads `DATABASE_URL` or builds DSN, opens pgx, runs `goose.Up`.

---

## 4. Backend -- Internal Layers (`internal/`)

### 4.1 Models (`internal/model/`) -- 21 GORM Models

| # | File | Struct | Table | Notable Fields |
|---|------|--------|-------|----------------|
| 1 | `user.go` | `User` | users | Role (superadmin/admin/cs/billing/technician/readonly), BearerKey |
| 2 | `customer.go` | `Customer` | customers | CustomerCode (unique), Tags (jsonb), PortalPasswordHash |
| 3 | `subscription.go` | `Subscription` | subscriptions | Status state machine (pending/active/suspended/isolated/expired/terminated), BillingDay, AutoIsolate |
| 4 | `invoice.go` | `Invoice` | invoices | InvoiceNumber (unique), Status (draft/sent/unpaid/partial/paid/overpaid/overdue/cancelled/refunded), InvoiceType (recurring/installation/additional/refund) |
| 5 | `invoice_item.go` | `InvoiceItem` | invoice_items | ItemType, IsProrated, ProrationDays |
| 6 | `payment.go` | `Payment` | payments | PaymentMethod (cash/bank_transfer/e-wallet/credit_card/debit_card/check/qris/gateway), Gateway fields (Name/TrxID/Response/PaymentURL) |
| 7 | `payment_allocation.go` | `PaymentAllocation` | payment_allocations | PaymentID, InvoiceID, AllocatedAmount |
| 8 | `bandwidth_profile.go` | `BandwidthProfile` | bandwidth_profiles | DownloadSpeed, UploadSpeed, PriceMonthly, RateLimit, IsolateProfileName |
| 9 | `bandwidth_config.go` | `PPPProfileConfig` | -- (virtual) | LocalAddress, RemoteAddress, DNSServer, RateLimit, Bridge |
| 10 | `mikrotik_router.go` | `MikrotikRouter` | mikrotik_routers | PasswordEncrypted, IsMaster, Status (online/offline/unknown) |
| 11 | `customer_registration.go` | `CustomerRegistration` | customer_registrations | Status (pending/approved/rejected), BandwidthProfileID |
| 12 | `system_setting.go` | `SystemSetting` | system_settings | GroupName+KeyName (unique composite), Type (string/integer/boolean/json/password), IsEncrypted |
| 13 | `message_template.go` | `MessageTemplate` | message_templates | Event, Channel (whatsapp/email), Body with placeholders |
| 14 | `sequence_counter.go` | `SequenceCounter` | sequence_counters | Prefix, Padding, LastNumber, ResetMonthly, ResetYearly |
| 15 | `audit_log.go` | `AuditLog` | audit_logs | OldValue/NewValue (jsonb), EntityType, EntityID |
| 16 | `hotspot_sale.go` | `HotspotSale` | hotspot_sales | BatchCode, SalesAgentID, Profile, Price, SellingPrice |
| 17 | `sales_agent.go` | `SalesAgent` | sales_agents | VoucherMode (mix/num/alp), VoucherType (upp/up), BillingCycle, BillingDay |
| 18 | `sales_agent.go` | `SalesProfilePrice` | sales_profile_prices | BasePrice, SellingPrice, VoucherLength |
| 19 | `agent_invoice.go` | `AgentInvoice` | agent_invoices | VoucherCount, Subtotal, SellingTotal, Profit, Status (draft/unpaid/paid/cancelled) |
| 20 | `cash_entry.go` | `CashEntry` | cash_entries | Type (income/expense), Source, PettyCashFundID, Status |
| 21 | `petty_cash_fund.go` | `PettyCashFund` | petty_cash_funds | InitialBalance, CurrentBalance, CustodianID |
| 22 | `casbin_rule.go` | `CasbinRule` | casbin_rule | Ptype, V0-V5 |

### 4.2 Domain Layer (`internal/domain/`) -- 7 Modules

Each domain module encapsulates **pure business logic** with no external dependencies (no database, no HTTP). This enables unit testing without mocks.

| Module | File(s) | Key Methods |
|--------|---------|-------------|
| `billing` | `billing.go` | `CalculateTax`, `CalculateTotal`, `CalculateProration`, `CalculateLateFee`, `IsOverdue`, `DaysOverdue`, `ShouldSuspendForNonPayment`, `ShouldSendReminder`, `InvoiceStatusFromAmounts`, `ClampBillingDay`, `ResolveGracePeriod`, `GetBillingPeriod` |
| `payment` | `payment.go` | `ValidatePayment`, `CanConfirm`, `CanReject`, `CanRefund`, `IsGatewayPayment`, `CalculateAllocations` (FIFO) |
| `subscription` | `subscription.go` | `ValidateStatusTransition`, `CanActivate/Suspend/Isolate/Restore/Terminate`, `IsExpired`, `NeedsSync`, `ValidateCredentials`, `GeneratePassword` |
| `customer` | `customer.go` | `ValidateCustomer`, `CanDeactivate`, `CanActivate` |
| `router` | `router.go` | `ValidateConnection`, `IsOnline`, `CanConnect`, `ShouldSync`, `IsStale` |
| `notification` | `notification.go` | `RenderTemplate`, `RenderSubject`, `ShouldSend`, `ExtractPlaceholders`, `ValidateTemplate` |
| `registration` | `registration.go` | `ValidateRegistration`, `CanApprove`, `CanReject`, `IsAlreadyConverted`, `NeedsProfileAssignment` |

**Subscription State Machine:**

```
pending --> active --> suspended --> isolated --> expired
  |                    |               |             |
  +---> terminated <---+---------------+-------------+
                         ^                 |
                         +---- restored ----+
```

### 4.3 Service Layer (`internal/service/`) -- 13 Core Services

| Service | File | Dependencies | Key Behavior |
|---------|------|-------------|--------------|
| `AuthService` | `auth_service.go` | UserRepo, JWT, Redis | Login/logout, token refresh, password change, session management |
| `CustomerService` | `customer_service.go` | CustomerRepo, SequenceCounterRepo, BandwidthProfileRepo, CustomerDomain | Auto-generates CustomerCode (CST-xxxxx) |
| `SubscriptionService` | `subscription_service.go` | SubscriptionRepo, BandwidthProfileRepo, SystemSettingRepo, SubscriptionDomain, RouterService, Redis | Full lifecycle management, PPP secret sync to MikroTik |
| `BillingService` | `billing_service.go` | InvoiceRepo, InvoiceItemRepo, SubscriptionRepo, BandwidthProfileRepo, CustomerRepo, SystemSettingRepo, SequenceCounterRepo, BillingDomain | Daily billing, overdue isolation, payment reminders |
| `PaymentService` | `payment_service.go` | PaymentRepo, InvoiceRepo, PaymentAllocationRepo, CustomerRepo, SequenceCounterRepo, PaymentDomain, BillingDomain, Transactor | FIFO allocation, transactional payment+invoice update |
| `RouterService` | `router_service.go` | RouterDeviceRepo, encryption key, Redis | Router CRUD, connection testing, health sync, password encryption (AES-GCM) |
| `RegistrationService` | `registration_service.go` | CustomerRegistrationRepo, CustomerService, SubscriptionService | Approve creates customer + subscription atomically |
| `NotificationService` | `notification_service.go` | MessageTemplateRepo, SystemSettingRepo, WhatsAppSender | Template rendering with placeholder substitution |
| `ReportService` | `report_service.go` | DB (raw SQL) | Subscription stats, report summary, cash flow |
| `BandwidthProfileService` | `bandwidth_profile_service.go` | BandwidthProfileRepo, RouterService, Redis | Profile CRUD with MikroTik PPP profile sync |
| `HotspotSaleService` | `hotspot_sale_service.go` | HotspotSaleRepo, SalesAgentRepo | Sale tracking with batch codes |
| `AgentInvoiceService` | `agent_invoice_service.go` | AgentInvoiceRepo, HotspotSaleRepo, SalesAgentRepo, SequenceCounterRepo | Agent invoice generation from hotspot sales |
| `CashManagementService` | `cash_management_service.go` | CashEntryRepo, PettyCashFundRepo, SequenceCounterRepo, DB | Cash flow reports, reconciliation, fund management |

Plus 9 Mikrotik sub-services (`internal/service/mikrotik/`): PPP, Hotspot, Queue, Firewall, IPPool, IPAddress, Monitor, Report, Script.

Plus 5 Mikhmon sub-services (`internal/service/mikrotik/mikhmon/`): Voucher, Profile, Report, Generator, Expire.

### 4.4 Repository Layer (`internal/repository/`) -- 19 Interfaces

| Interface | File | Key Methods |
|-----------|------|-------------|
| `UserRepository` | `user_repo.go` | Create, GetByID, GetByEmail, GetByRole, Update, Delete, List, Count, UpdateLastLogin |
| `CustomerRepository` | `customer_repo.go` | Create, GetByID/Email/Username, Update, Delete, List, Count |
| `SubscriptionRepository` | `subscription_repo.go` | Create, GetByID/CustomerID/Username, Update, Delete, List, Count, ListByRouterID, ListByStatus, UpdateStatus |
| `InvoiceRepository` | `invoice_repo.go` | Create, GetByID/CustomerID, Update, UpdateStatus, Delete, List, GetOverdue, GetBySubscriptionAndPeriod |
| `InvoiceItemRepository` | `invoice_item_repo.go` | Create, GetByID, Update, Delete, ListByInvoiceID, DeleteByInvoiceID |
| `PaymentRepository` | `payment_repo.go` | Create, GetByID/InvoiceID/TransactionID/CustomerID, Update, UpdateStatus, List |
| `PaymentAllocationRepository` | `payment_allocation_repo.go` | Create, ListByPaymentID, ListByInvoiceID |
| `BandwidthProfileRepository` | `bandwidth_profile_repo.go` | Create, GetByID/Code, GetByRouterAndCode/Name, ListActive, ListActiveByRouterID |
| `RouterDeviceRepository` | `router_device_repo.go` | Create, GetByID, GetActive, Update, UpdateLastSync, Delete, List |
| `CustomerRegistrationRepository` | `customer_registration_repo.go` | Create, GetByID, Update, ListByStatus, UpdateStatus |
| `SystemSettingRepository` | `system_setting_repo.go` | GetByGroupAndKey, Upsert, ListByGroup |
| `MessageTemplateRepository` | `message_template_repo.go` | GetByEventAndChannel, ListActive, ListByEvent |
| `SequenceCounterRepository` | `sequence_counter_repo.go` | GetByName, NextNumber (atomic increment) |
| `AuditLogRepository` | `audit_log.go` | Create, ListByEntity, ListByAdmin |
| `HotspotSaleRepository` | `hotspot_sale_repo.go` | CreateBatch, List (filtered), SumByAgentAndPeriod, DeleteByBatchCode |
| `SalesAgentRepository` | `hotspot_sale_repo.go` | Create, GetByUsername, UpsertProfilePrice, ListProfilePrices |
| `AgentInvoiceRepository` | `agent_invoice_repo.go` | Create, GetByAgentAndPeriod, UpdateStatus, List (filtered), GetUnpaidOverdue |
| `CashEntryRepository` | `cash_entry_repo.go` | List (filtered), SumByTypeAndPeriod, SumBySourceAndPeriod |
| `PettyCashFundRepository` | `petty_cash_fund.go` | Create, AdjustBalance |
| `Transactor` | `transactor.go` | `RunInTx(ctx, func(txPayment, txInvoice, txAlloc))` -- explicit transaction boundaries |

All PostgreSQL implementations live in `internal/repository/postgres/` (one file per interface).

### 4.5 Handler Layer (`internal/handler/`) -- 32 Handler Structs

#### Core Handlers

| Handler | File | Routes |
|---------|------|--------|
| `AuthHandler` | `auth_handler.go` | Login, Logout, RefreshToken, ChangePassword, GetMe |
| `UserHandler` | `user_handler.go` | CRUD for admin users |
| `CustomerHandler` | `customer_handler.go` | CRUD + ActivateAccount/DeactivateAccount |
| `SubscriptionHandler` | `subscription_handler.go` | CRUD + Activate/Suspend/Isolate/Restore/Terminate |
| `BillingHandler` | `billing_handler.go` | ListInvoices, GetInvoice, GetOverdue, TriggerMonthlyBilling |
| `PaymentHandler` | `payment_handler.go` | CRUD + Confirm/Reject/Refund/InitiateGateway |
| `RouterHandler` | `router_handler.go` | CRUD + Sync/TestConnection/SyncAll/SelectRouter |
| `RegistrationHandler` | `registration_handler.go` | List/Get/Approve/Reject |
| `WebhookHandler` | `webhook_handler.go` | Xendit/Midtrans webhook receivers |
| `SystemSettingHandler` | `system_setting_handler.go` | List/Get/Upsert |
| `BandwidthProfileHandler` | `bandwidth_profile_handler.go` | CRUD |
| `ReportHandler` | `report_handler.go` | Summary/Subscription reports |
| `CustomerPortalHandler` | `customer_portal_handler.go` | Login/Profile/Invoices/Payments/Gateway |
| `HotspotSaleHandler` | `hotspot_sale_handler.go` | List/ListByRouter |
| `SalesAgentHandler` | `sales_agent_handler.go` | CRUD + ProfilePrices |
| `AgentInvoiceHandler` | `agent_invoice_handler.go` | List/Generate/MarkPaid/Cancel/ProcessScheduled |
| `AgentPortalHandler` | `agent_portal_handler.go` | Login/Profile/Invoices/Sales |
| `CashManagementHandler` | `cash_management_handler.go` | CashEntry CRUD + Approve/Reject + PettyCashFund CRUD + TopUp + CashFlow/Balance/Reconciliation reports |

#### MikroTik Sub-handlers (`internal/handler/mikrotik/`)

| Handler | File | Purpose |
|---------|------|---------|
| `PPPHandler` | `ppp_handler.go` | PPP profile/secret CRUD, list active |
| `PPPWSHandler` | `ppp_ws_handler.go` | WebSocket: listen active/inactive PPP sessions |
| `HotspotHandler` | `hotspot_handler.go` | Hotspot profile/user CRUD, list active/hosts/servers |
| `HotspotWSHandler` | `hotspot_ws_handler.go` | WebSocket: listen hotspot active/inactive |
| `IPHandler` | `ip_handler.go` | IP pool + address management |
| `FirewallHandler` | `firewall_handler.go` | Filter/NAT rules, address lists |
| `MonitorHandler` | `monitor_handler.go` | System resource, interface list |
| `MonitorWSHandler` | `monitor_ws_handler.go` | WebSocket: system resource, traffic, logs, ping |
| `QueueHandler` | `queue_handler.go` | Simple queue listing |
| `RawHandler` | `raw_handler.go` | Arbitrary RouterOS commands + WebSocket listen |

#### Mikhmon Sub-handlers (`internal/handler/mikrotik/mikhmon/`)

| Handler | File | Purpose |
|---------|------|---------|
| `VoucherHandler` | `voucher_handler.go` | Generate batch, list vouchers, remove batch |
| `ProfileHandler` | `profile_handler.go` | Create/update Mikhmon profiles |
| `ExpireHandler` | `expire_handler.go` | Setup/disable expiration monitor |
| `ReportHandler` | `report_handler.go` | Add/get reports, get summary |

### 4.6 Middleware (`internal/middleware/`) -- 9 Middlewares

| Middleware | File | Purpose |
|------------|------|---------|
| `AuthMiddleware` | `auth.go` | JWT validation (HS256), Redis session check, role-based authorization |
| `PortalAuthMiddleware` | `portal_auth.go` | Customer portal JWT validation |
| `AgentPortalAuthMiddleware` | `agent_portal_auth.go` | Agent portal JWT validation |
| `CasbinMiddleware` | `casbin_middleware.go` | RBAC enforcement via Casbin (extracts role from JWT, checks policy) |
| `RateLimitMiddleware` | `ratelimit.go` | Redis sliding window rate limiter |
| `CORSMiddleware` | `cors.go` | CORS with configurable origins |
| `LoggerMiddleware` | `logger.go` | Zap-based request logging |
| `RequestIDMiddleware` | (gin-contrib/requestid) | X-Request-ID header |
| `MikrotikRouterMiddleware` | `mikrotik_router.go` | Validates `:router_id` param, injects MikroTik client |

### 4.7 Queue System (`internal/queue/`) -- 3 Producers + 3 Consumers

| Producer | Exchange | Routing Key | Event |
|----------|----------|-------------|-------|
| `BillingProducer` | `billing.exchange` | `invoice.generate` | `GenerateInvoiceEvent{CustomerID, PackageID, Amount, Period}` |
| `SuspendProducer` | `suspend.exchange` | `customer.suspend` | `SuspendCustomerEvent{CustomerID, RouterID, Reason}` |
| `NotificationProducer` | `notification.exchange` | `notification.send` | `NotificationEvent{Type, To, Subject, Body}` |

| Consumer | Queue | Handler Callback |
|----------|-------|-----------------|
| `BillingConsumer` | `billing.queue` | `BillingHandler(ctx, subscriptionID, period)` |
| `SuspendConsumer` | `suspend.queue` | `SuspendHandler(ctx, customerID)` |
| `NotificationConsumer` | `notification.queue` | `NotificationHandler{SendWhatsApp, SendEmail}` |

### 4.8 Scheduler (`internal/scheduler/`) -- 6 Cron Jobs

| Job | Schedule | Action |
|-----|----------|--------|
| Daily Billing | `0 0 * * *` (00:00) | `BillingService.ProcessDailyBilling()` -- generate subscription invoices |
| Agent Invoice | `5 0 * * *` (00:05) | `AgentInvoiceService.ProcessScheduled()` -- generate agent invoices |
| Overdue Isolation | `0 1 * * *` (01:00) | `BillingService.CheckAndIsolateOverdue()` |
| Isolation Check | `0 2 * * *` (02:00) | `BillingService.CheckAndIsolateOverdue()` |
| Payment Reminder | `0 8 * * *` (08:00) | `BillingService.CheckAndSendReminders()` -- WhatsApp notifications |
| Router Sync | `*/5 * * * *` (every 5 min) | `RouterService.SyncAllDevices()` -- health check + status update |

### 4.9 Notification (`internal/notification/`)

| Interface | Method |
|-----------|--------|
| `WhatsAppSender` | `SendMessage(ctx, phone, message)`, `SendGroupMessage(ctx, message)` |
| `EmailSender` | `SendEmail(ctx, to, subject, body)` |

| Implementation | Description |
|---------------|-------------|
| `GoWAClient` | Wraps `pkg/gowa.Client`, normalizes Indonesian phone numbers (0xxx -> 62xxx) |
| `EmailClient` | SMTP with TLS (port 465) or STARTTLS fallback |

### 4.10 Seeder (`internal/seeder/`) -- 7 Seed Types

| Seed | Data |
|------|------|
| Users | 5 admins: superadmin, admin, cs, billing, technician |
| Routers | 2 routers: "Router Utama" (192.168.233.1), "Router 2" (192.168.27.1) |
| Customers | 5 sample customers (Budi Santoso, Siti Rahayu, etc.) |
| Casbin Rules | 4 role groupings + 10 policies |
| System Settings | 9 settings (isolate profile, notification URLs, billing config) |
| Message Templates | 11 templates across 8 events (invoice, payment, isolation, registration, agent) |
| Sequence Counters | 4 counters: INV-, PAY-, CST-, KAS- |

All seeds are idempotent (`ON CONFLICT DO NOTHING` / `WHERE NOT EXISTS`).

### 4.11 Migrations (`internal/migration/`) -- 25 Files

Goose v3 Go-based migrations, numbered 001-029 (some gaps). Creates all 15+ tables and seeds initial system data.

### 4.12 Casbin RBAC (`internal/casbin/`)

**Model** (`model.conf`): RBAC with regex matching.
```
[request_definition]  r = sub, obj, act
[policy_definition]   p = sub, obj, act
[role_definition]     g = _, _
[policy_effect]       e = some(where (p.eft == allow))
[matchers]            m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
```

**Default Role Hierarchy:**
- `superadmin` inherits `admin`
- `cs`, `billing`, `technician` inherit `staff`

**Default Policies:**
- `admin` -> `/api/v1/*` -> `.*` (full access)
- `staff` -> limited paths (invoices, payments, customers, registrations, reports) -> GET/POST/PUT/DELETE

---

## 5. Backend -- Shared Libraries (`pkg/`)

### 5.1 `pkg/mikrotik/` -- Custom RouterOS API Library (90+ files)

The most critical package. A full-featured MikroTik RouterOS API client with a 3-pipeline collector system, multi-router support, and repository pattern.

#### Client Layer (`pkg/mikrotik/client/`)

| File | Key Types | Purpose |
|------|-----------|---------|
| `options.go` | `Config{Host, Port, Username, Password, UseTLS, ReconnectInterval, Timeout}` | Connection configuration |
| `client.go` | `Client` | Async RouterOS client with auto-reconnect (exponential backoff 1s-30s), concurrent command execution (`RunMany`), multi-stream fan-in (`ListenManyArgsContext`), raw map output (`RunRaw`) |
| `manager.go` | `Manager` | Multi-router connection manager: `Register`, `Get`, `GetOrConnect`, `Unregister`, `TestConnection`, `CloseAll` |
| `helpers.go` | Parsing functions | `ParseRate` (kbps/Mbps), `SplitSlashValue`, `SplitRateValue`, `ParseByteSize` (MiB/KiB), `ListenBatches` (debounced streaming) |

#### Domain Entities (`pkg/mikrotik/domain/`)

| File | Entities |
|------|----------|
| `hotspot.go` | `HotspotUser`, `HotspotActive`, `HotspotHost`, `HotspotProfile` (25+ fields), `HotspotIPBinding`, `HotspotCookie` |
| `ppp.go` | `PPPSecret`, `PPPProfile`, `PPPActive` + request/update structs |
| `system.go` | `SystemResource`, `SystemHealth`, `SystemIdentity`, `SystemClock`, `RouterBoardInfo`, `LogEntry` |
| `queue.go` | `QueueStats` (20+ fields), `SimpleQueue`, `TreeQueue` |
| `firewall.go` | `NATRule`, `FirewallRule`, `AddressList` |
| `interface.go` | `Interface`, `TrafficStats`, `TrafficMonitorStats` |
| `voucher.go` | `VoucherGenerateRequest`, `Voucher`, `PrintVoucherRequest` |
| `report.go` | `SalesReport`, `ReportFilter`, `ReportSummary`, `VoucherSaleRecord` |
| `ping.go` | `PingConfig`, `PingResult` |
| `ip.go` | `IPAddress`, `IPPool`, `IPPoolUsed` |
| `errors.go` | `ErrConnectionFailed`, `ErrAuthentication`, `ErrCommandFailed`, `ErrTimeout` |
| `mikhmon/` | Sub-package with voucher, report, profile, generator domain models |

#### Repository Layer (`pkg/mikrotik/repository/`)

Each module follows interface + implementation + aggregator pattern:

| Module | Interface Methods |
|--------|------------------|
| `hotspot` | `UserRepository` (15 methods), `ProfileRepository` (11), `ActiveRepository` (5), `HostRepository` (2), `IPBindingRepository` (5), `ServerRepository` (1) |
| `ppp` | `SecretRepository` (11 methods), `ProfileRepository` (11), `ActiveRepository` (5) |
| `system` | `IdentityRepository`, `ResourcesRepository`, `RouterBoardRepository`, `SchedulerRepository`, `ScriptsRepository` |
| `monitor` | `SystemRepository`, `InterfaceRepository`, `PingRepository`, `LogRepository` |
| `firewall` | `NATRepository`, `FilterRepository`, `AddressListRepository` |
| `queue` | `SimpleQueueRepository` (11 methods), `StatsRepository` |
| `ip-address` | `AddressRepository` (8 methods), `PoolRepository` (7) |
| `mikhmon` | `VoucherRepository`, `ProfileRepository`, `ReportRepository`, `GeneratorRepository`, `ExpireRepository` |

#### 3-Pipeline Collector System (`pkg/mikrotik/collector/`)

```
Manager (manages all routers)
  |
  +-> Supervisor (per router, health check loop every 30s)
       |
       +-> Pool (per pipeline type)
       |    +-> TimeSeries: 5 conns, 20 cmds max
       |    +-> Operational: 5 conns, 10 cmds max
       |    +-> OnDemand: 2 conns, 25 cmds max
       |
       +-> Pipeline A: time_series.Collector
       |    |   Specs: interface_traffic, queue_stats, system_resource
       |    |   Uses: ListenRaw (streaming) -> MetricEvent channel (buffer 1000)
       |    v   Dest: BatchWriter -> InfluxHandler -> InfluxDB 3
       |
       +-> Pipeline B (Tier2): operational.Tier2Collector
       |    |   Specs: ppp_active, hotspot_active, interface_status
       |    |   Uses: follow=yes (real-time event-driven) -> StateEvent channel
       |    v   Dest: BatchWriter -> RedisHandler -> Redis HSET (no TTL)
       |
       +-> Pipeline B (Tier3): operational.Tier3Collector
       |    |   Specs: ppp_secrets (5min), ppp_profiles (5min), hotspot_users (5min), ip_pools (10min)
       |    |   Uses: RunRaw on interval ticker (minimum 10s)
       |    v   Dest: BatchWriter -> RedisHandler -> Redis HSET (with TTL from spec)
       |
       +-> Pipeline C: ondemand.Runner
            |   Direct read/write with cache-through
            v   Read: check Redis cache first, miss -> RouterOS API
                Write: RouterOS API + Redis cache invalidation
```

**`CommandSpec`** (`collector/spec.go`):
```go
type CommandSpec struct {
    Name        string       // e.g., "interface_traffic"
    Pipeline    PipelineType // "time_series" | "operational" | "on_demand"
    Tier        Tier         // 2 (event-driven) | 3 (static ticker)
    Storage     StorageType  // "influxdb" | "redis" | "none"
    Args        []string     // RouterOS command args
    UseFollow   bool         // true for follow=yes streaming
    Interval    time.Duration
    Measurement string       // InfluxDB measurement name
    TagFields   []string     // InfluxDB tags
    ValueFields []string     // InfluxDB fields
    RedisKey    string       // Redis hash key pattern
    KeyField    string       // Redis hash field
    TTL         time.Duration
}
```

**BatchWriter** (`collector/writer/batch_writer.go`):
- Shared batching layer for both InfluxDB and Redis
- Configurable: `BatchSize` (default 100), `FlushInterval` (50ms), `MaxRetries` (3)
- Dual handler: `InfluxHandler` + `RedisHandler` both implement `BatchHandler` interface

**10 Default Specs** returned by `DefaultSpecs()` + `Tier3Specs()`:

| Spec | Pipeline | Tier | Storage | Interval |
|------|----------|------|---------|----------|
| `interface_traffic` | A (time_series) | -- | InfluxDB | streaming |
| `queue_stats` | A (time_series) | -- | InfluxDB | streaming |
| `system_resource` | A (time_series) | -- | InfluxDB | streaming |
| `ppp_active` | B (operational) | Tier 2 (follow) | Redis | real-time |
| `hotspot_active` | B (operational) | Tier 2 (follow) | Redis | real-time |
| `interface_status` | B (operational) | Tier 2 (follow) | Redis | real-time |
| `ppp_secrets` | B (operational) | Tier 3 (ticker) | Redis | 5 min |
| `ppp_profiles` | B (operational) | Tier 3 (ticker) | Redis | 5 min |
| `hotspot_users` | B (operational) | Tier 3 (ticker) | Redis | 5 min |
| `ip_pools` | B (operational) | Tier 3 (ticker) | Redis | 10 min |

### 5.2 Other `pkg/` Libraries

| Package | File(s) | Key Types/Functions |
|---------|---------|---------------------|
| `pkg/jwt/` | `jwt.go` | `Service` with `Generate`, `GenerateRefreshToken`, `GenerateTokenPair`, `GeneratePortal`, `GenerateAgent`, `Validate`. Claims: UserID, Email, Role, TokenType (access/refresh/portal/agent_portal). HS256 signing. |
| `pkg/redis/` | 6 files | `Client` with Cache (Get/Set/Del/Exists/TTL), Session (SetSession/GetSession/DeleteSession/BlacklistToken), RateLimit (sliding window), PubSub, Hash operations. Key prefixes: `session:`, `blacklist:`, `pwd_changed:`, `selected_router:` |
| `pkg/rabbitmq/` | 5 files | `Client` with Publish (persistent, 5s timeout), PublishWithDelay, Subscribe (auto-ack/nack+requeue), SubscribeWithQOS, DeclareExchange, DeclareQueue, BindQueue |
| `pkg/logger/` | `logger.go` | `Logger` wrapping `zap.Logger`. Production: JSON+ISO8601. Development: colored console. Both with caller info. |
| `pkg/response/` | `response.go` | `Success`, `Error`, `WithMeta`, `OK`, `Created`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `InternalServerError` -- standardized Gin JSON responses |
| `pkg/validator/` | `validator.go` | Wraps `go-playground/validator/v10`. Struct validation + custom rule registration |
| `pkg/pagination/` | `pagination.go` | `Params{Limit, Offset, Page}`, `Meta{Total, Limit, Offset, Page, Pages}`, `FromContext(c)` reads query params |
| `pkg/payment/` | 4 files | `Provider` interface (`Name`, `CreateInvoice`, `VerifyWebhook`). Xendit (active), Tripay (stub), Midtrans (stub). Status mapping: PAID/SETTLED->confirmed, EXPIRED/FAILED->rejected |
| `pkg/gowa/` | 8 files | Full GoWA WhatsApp client. Multi-device support. Login (QR + pairing code), send text/image/file/video, group management, user check, contact list. HTTP Basic Auth + X-Device-Id header. |
| `pkg/ws/` | 2 files | `Upgrader` with configurable origins. `StreamConfig` + `ForwardChannel[T]` -- generic type-safe WebSocket streaming. Interface name validation regex. |

---

## 6. Frontend (`web/`)

### 6.1 Build Configuration

| File | Key Settings |
|------|-------------|
| `package.json` | Based on shadcn-admin v2.2.1 template. React 19, Vite 8 (SWC), TanStack Router/Query/Table, Zustand, Zod, Axios, Radix UI, Tailwind v4, Recharts |
| `vite.config.ts` | TanStack Router plugin (auto code splitting), Tailwind v4, `@/` path alias, dev proxy: `/api`, `/portal`, `/agent-portal` -> `localhost:8080` (WebSocket enabled) |
| `components.json` | Shadcn UI: new-york style, slate base, TSX, Lucide icons |
| `eslint.config.js` | typescript-eslint, react-hooks, TanStack query plugin, no-console error, type-only imports |

### 6.2 Feature Modules (15 modules)

| Module | Path | Components |
|--------|------|-----------|
| **auth** | `features/auth/` | sign-in (admin), customer-login, agent-login, sign-up, OTP, forgot-password, change-password |
| **dashboard** | `features/dashboard/` | analytics-chart, kpi-card, overview, recent-activity-feed, recent-sales, revenue-chart, router-health-cards |
| **customers** | `features/customers/` | customer-table, registration-table, create/edit/delete/approve/reject dialogs |
| **routers** | `features/routers/` | router-table, create/edit/delete dialogs |
| **profiles** | `features/profiles/` | profile-table, create/edit dialogs (bandwidth profiles) |
| **billing/invoices** | `features/billing/invoices/` | invoice-table, invoice-detail-sheet, invoice-generation-trigger |
| **billing/payments** | `features/billing/payments/` | payment-table, confirm/reject/refund dialogs, date-range-filter |
| **billing/cash** | `features/billing/cash/` | cash-table, create-cash-entry, cash-reject, petty-cash-card, top-up dialogs |
| **mikhmon/hotspot** | `features/mikhmon/hotspot/` | users/profiles/active/hosts tables with CRUD dialogs |
| **mikhmon/vouchers** | `features/mikhmon/vouchers/` | vouchers-table, generate-voucher, delete-voucher, expire-monitor dialogs |
| **mikhmon/report** | `features/mikhmon/report/` | report-table, report-summary |
| **customer-portal** | `features/customer-portal/` | subscriptions, payments, invoices views |
| **settings** | `features/settings/` | account, appearance, display, notifications, profile forms |
| **reports** | `features/reports/` | business reports view |
| **errors** | `features/errors/` | 401, 403, 404, 500, 503 error pages |

### 6.3 API Client Modules (26 source files, 18 modules)

| Module | Functions |
|--------|-----------|
| `api/auth.ts` | adminLogin, adminRefreshToken, adminChangePassword, adminLogout, adminGetMe, customerLogin, agentLogin |
| `api/router.ts` | listRouters, getSelectedRouter, selectRouter, createRouter, syncRouter, testConnection, updateRouter, deleteRouter, syncAllRouters |
| `api/user.ts` | listUsers, getUser, createUser, deleteUser |
| `api/customer.ts` | CRUD + activateAccount, deactivateAccount, listRegistrations, approveRegistration, rejectRegistration, publicRegister |
| `api/subscription.ts` | CRUD + activate, suspend, isolate, restore, terminate |
| `api/invoice.ts` | listInvoices, listOverdueInvoices, getInvoice, deleteInvoice, triggerMonthlyBilling |
| `api/payment.ts` | CRUD + confirm, reject, refund, initiateGatewayPayment |
| `api/cash.ts` | CRUD + approve/reject for cash entries + petty cash funds + topUp |
| `api/report.ts` | getReportSummary, getSubscriptionReport, getCashFlowReport, getCashBalanceReport, getReconciliationReport |
| `api/settings.ts` | listSettings, upsertSetting, getSetting, updateSetting, deleteSetting |
| `api/profiles.ts` | CRUD for bandwidth profiles |
| `api/sales-agents.ts` | CRUD + getAgentProfilePrices, upsertAgentProfilePrice, generateAgentInvoice |
| `api/hotspot-sales.ts` | listHotspotSales, createHotspotSale, listRouterHotspotSales |
| `api/admin-agent-invoice.ts` | list, get, pay, cancel, processScheduled |
| `api/mikrotik/raw.ts` | runRawCommand |
| `api/mikrotik/monitor.ts` | getSystemResource, getInterfaces |
| `api/mikrotik/network.ts` | listQueues, listFirewallFilters, listFirewallNat, listFirewallAddressList, IP pool CRUD, listIpAddresses |
| `api/mikrotik/hotspot.ts` | Hotspot profile/user CRUD, listActive, listHosts, listServers |
| `api/mikrotik/ppp.ts` | PPP profile/secret CRUD, listActive |
| `api/mikrotik/mikhmon.ts` | generateVouchers, listVouchers, removeVoucherBatch, profile CRUD, report CRUD, expiration monitor |
| `api/portal/customer.ts` | getCustomerProfile, changeCustomerPassword |
| `api/portal/agent.ts` | getAgentProfile, changeAgentPassword |
| `api/portal/subscription.ts` | listPortalSubscriptions |
| `api/portal/payment.ts` | listPortalPayments, initiatePortalPayment |
| `api/portal/invoice.ts` | listPortalInvoices, getPortalInvoice |
| `api/portal/agent-invoice.ts` | listAgentPortalInvoices, getAgentPortalInvoice, requestAgentPayment |

### 6.4 Custom Hooks (20 files)

| Hook File | Exported Hooks |
|-----------|---------------|
| `use-admin-auth.ts` | useAdminLogin, useAdminLogout, useAdminChangePassword |
| `use-customer-auth.ts` | useCustomerLogin, useCustomerLogout, useCustomerUser |
| `use-agent-auth.ts` | useAgentLogin, useAgentLogout, useAgentUser |
| `use-customers.ts` | useCustomers, useCreateCustomer, useActivateCustomer, useDeactivateCustomer, useDeleteCustomer, useUpdateCustomer, useRegistrations, useApproveRegistration, useRejectRegistration |
| `use-payments.ts` | usePayments, useConfirmPayment, useRejectPayment, useRefundPayment, useInitiateGatewayPayment |
| `use-invoices.ts` | useInvoices, useOverdueInvoices, useInvoice, useTriggerMonthlyBilling |
| `use-cash.ts` | useCashEntries, useCreateCashEntry, useApproveCashEntry, useRejectCashEntry, usePettyCashFunds, useCreatePettyCashFund, useTopUpPettyCashFund |
| `use-routers.ts` | useRouters, useSelectRouter, useCreateRouter, useSyncRouter, useTestRouterConnection, useUpdateRouter, useDeleteRouter, useSyncAllRouters |
| `use-profiles.ts` | useProfiles, useCreateProfile, useDeleteProfile, useUpdateProfile |
| `use-subscriptions.ts` | useSubscriptions, useCreateSubscription, useActivateSubscription, useSuspendSubscription, useIsolateSubscription, useRestoreSubscription, useTerminateSubscription, useDeleteSubscription |
| `use-users.ts` | useUsers, useUser, useCreateUser, useDeleteUser |
| `use-hotspot.ts` | useHotspotProfiles/Users/Active/Hosts/Servers + CRUD mutations |
| `use-mikhmon.ts` | useMikhmonVouchers/Reports/Profiles + generate/remove/expiration hooks |
| `use-ping.ts` | usePing(routerId) -- WebSocket real-time ping with auto-reconnect |
| `use-report-summary.ts` | useReportSummary |
| `use-portal-billing.ts` | usePortalInvoices/Payments, useAgentPortalInvoices, useAgentRequestPayment |
| `use-customer-portal.ts` | usePortalSubscriptions |
| `use-dialog-state.tsx` | useDialogState\<T\>() -- toggle dialog state |
| `use-table-url-state.ts` | useTableUrlState -- syncs pagination/filters to URL |
| `use-mobile.tsx` | useIsMobile -- responsive breakpoint |

### 6.5 State Management

**Zustand Stores (2):**

| Store | Persist Key | State |
|-------|-------------|-------|
| `useAuthStore` | `mikmongo-auth` (localStorage) | 3 auth slices: admin (accessToken, refreshToken, user), customer (token, user), agent (token, user). Each with set/clear methods. `isHydrated` flag. |
| `useRouterStore` | `mikmongo-router` (localStorage) | selectedRouterId, selectedRouterName, isHydrated, setSelectedRouter, clearSelectedRouter |

**React Context (5):**

| Provider | Persist | State |
|----------|---------|-------|
| `ThemeProvider` | Cookie | mode (light/dark/system) |
| `FontProvider` | Cookie | font (inter/manrope/system) |
| `DirectionProvider` | -- | direction (ltr/rtl) |
| `LayoutProvider` | -- | sidebar variant + collapsible |
| `SearchProvider` | -- | Cmd+K command menu |

### 6.6 Axios Clients (3 instances)

| Client | Base URL | Auth | 401 Handling |
|--------|----------|------|-------------|
| `adminClient` | `/api/v1` | Bearer token interceptor | Silent refresh (queued concurrent requests), redirect to /sign-in |
| `customerClient` | `/portal/v1` | Bearer token interceptor | Clear auth, redirect to /customer/login |
| `agentClient` | `/agent-portal/v1` | Bearer token interceptor | Clear auth, redirect to /agent/login |

### 6.7 Zod Schemas (12 schema files)

Located in `web/src/lib/schemas/`: auth, billing, customer, mikrotik, mikhmon, report, router, sales-agent, settings, subscription, user.

---

## 7. Database Layer

### 7.1 PostgreSQL Schema

**15 tables** created via 25 Goose migrations (Go-based):

```
users                      -- Admin users with role-based access
customers                  -- ISP customers with portal credentials
subscriptions              -- Service subscriptions with lifecycle states
invoices                   -- Billing invoices
invoice_items              -- Line items per invoice
payments                   -- Payment records with multi-method support
payment_allocations        -- FIFO payment-to-invoice allocations
mikrotik_routers           -- Managed router devices
bandwidth_profiles         -- Service plans with speed/price
customer_registrations     -- Pre-approval registration queue
system_settings            -- Key-value configuration store
message_templates          -- Notification templates per event
sequence_counters          -- Atomic auto-incrementing number generators
audit_logs                 -- Admin action audit trail
casbin_rule                -- RBAC policy storage
sales_agents               -- Field sales agents
sales_profile_prices       -- Agent-specific pricing per profile
hotspot_sales              -- Hotspot voucher sales records
agent_invoices             -- Agent billing statements
cash_entries               -- Income/expense tracking
petty_cash_funds           -- Petty cash fund management
```

### 7.2 Redis Usage

| Key Pattern | Purpose | TTL |
|-------------|---------|-----|
| `session:{token}` | User session data | JWT expiry |
| `blacklist:{jti}` | Revoked tokens | JWT expiry |
| `pwd_changed:{userID}` | Password change timestamp | 7 days |
| `selected_router:{userID}` | Admin's selected router | 24h |
| `ratelimit:{key}` | Rate limit counter | Window duration |
| `mikmongo:{routerID}:{key}:{field}` | Collector operational cache | Per-spec TTL or none |
| Pub/Sub channels | Real-time data streaming | -- |

### 7.3 InfluxDB 3 Usage

Time-series storage for Pipeline A metrics:

| Measurement | Tags | Fields |
|-------------|------|--------|
| `interface_traffic` | router_id, interface_name | rx_bits, tx_bits, rx_packets, tx_packets, rx_drops, tx_drops |
| `queue_stats` | router_id, queue_name | bytes_in, bytes_out, packets, rate, dropped |
| `system_resource` | router_id | cpu_load, free_memory, total_memory, uptime, cpu_count, version |

---

## 8. Infrastructure

### 8.1 Docker Compose (`deployments/docker-compose.yml`)

| Service | Image | Ports | Credentials | Volume |
|---------|-------|-------|-------------|--------|
| `postgres` | postgres:17-alpine | 5432:5432 | User/Pass/DB: mikmongo | postgres_data |
| `redis` | redis:7-alpine | 6379:6379 | None | redis_data |
| `rabbitmq` | rabbitmq:management-alpine | 5672:5672, 15672:15672 | User/Pass: mikmongo | rabbitmq_data |

Commented out: `app` (Go server, port 8080), `nginx` (reverse proxy, port 80).

### 8.2 Monitoring Stack (`deployments/docker-compose.monitor.yml`)

| Service | Image | Ports | Notes |
|---------|-------|-------|-------|
| `influxdb` | influxdb:3-core | 8181:8181 | File object store, healthcheck on /health |
| `redis` | redis:7-alpine | 6380:6379 | Separate instance on port 6380 for monitoring |

### 8.3 Dockerfile (`deployments/Dockerfile`)

```dockerfile
# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Stage 2: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
```

### 8.4 Nginx (`deployments/nginx.conf`)

Reverse proxy on port 80 -> app:8080 with WebSocket upgrade support for `/api/v1/ws/*`.

### 8.5 Environment Variables (`.env.example`)

85 lines covering: App (3), Database (7), Redis (4), RabbitMQ (4), JWT (3), MikroTik (4), Midtrans (3), Xendit (3), GoWA (6), Internal (1), Test (11).

### 8.6 Makefile (908 lines)

| Category | Targets |
|----------|---------|
| Docker | build, http, docker-build, docker-up, docker-down |
| Database | migrate-up, migrate-down, migrate-status, migrate-reset, seed, fresh |
| Scaffolding | model, domain, service, handler, repository, migration, scheduler, queue-producer, queue-consumer, mikrotik-domain, mikrotik-module |
| Dev | generate-mocks, tidy, lint, test, test-coverage, test-integration, dev (air), run (fzf) |

---

## 9. API Routes

### 9.1 Authentication Tiers

| Tier | Auth Method | Routes |
|------|-------------|--------|
| **Public** | None | Health, login, register, webhooks, portal login |
| **BearerAuth** | JWT (admin) + Casbin RBAC | `/api/v1/*` |
| **PortalAuth** | JWT (customer) | `/portal/v1/*` |
| **AgentPortalAuth** | JWT (agent) | `/agent-portal/v1/*` |

### 9.2 Public Routes

| Method | Path | Handler |
|--------|------|---------|
| GET | `/health` | Health check |
| POST | `/api/v1/auth/login` | AuthHandler.Login |
| POST | `/api/v1/auth/refresh` | AuthHandler.RefreshToken |
| POST | `/api/v1/register` | RegistrationHandler.Create |
| POST | `/api/v1/webhooks/midtrans` | WebhookHandler.MidtransWebhook |
| POST | `/api/v1/webhooks/xendit` | WebhookHandler.XenditWebhook |
| POST | `/portal/v1/login` | CustomerPortalHandler.Login |

### 9.3 Admin API Routes (`/api/v1` -- JWT + RBAC)

**Auth:**

| Method | Path | Handler |
|--------|------|---------|
| GET | `/auth/me` | AuthHandler.GetMe |
| POST | `/auth/change-password` | AuthHandler.ChangePassword |
| POST | `/auth/logout` | AuthHandler.Logout |

**Users:**

| Method | Path | Handler |
|--------|------|---------|
| GET/POST | `/users` | UserHandler.List/Create |
| GET/DELETE | `/users/:id` | UserHandler.Get/Delete |

**Customers:**

| Method | Path | Handler |
|--------|------|---------|
| GET/POST | `/customers` | CustomerHandler.List/Create |
| GET/PUT/DELETE | `/customers/:id` | CustomerHandler.Get/Update/Delete |
| POST | `/customers/:id/activate-account` | CustomerHandler.ActivateAccount |
| POST | `/customers/:id/deactivate-account` | CustomerHandler.DeactivateAccount |

**Routers (`/routers`):**

| Method | Path | Handler |
|--------|------|---------|
| GET | `/routers` | RouterHandler.List |
| POST | `/routers` | RouterHandler.Create |
| GET | `/routers/selected` | RouterHandler.GetSelectedRouter |
| POST | `/routers/select/:id` | RouterHandler.SelectRouter |
| POST | `/routers/sync-all` | RouterHandler.SyncAll |
| GET/PUT/DELETE | `/routers/:router_id` | RouterHandler.GetDevice/Update/Delete |
| POST | `/routers/:router_id/sync` | RouterHandler.SyncDevice |
| POST | `/routers/:router_id/test-connection` | RouterHandler.TestConnection |

**Per-Router Sub-resources (`/routers/:router_id/...`):**

| Sub-resource | Routes |
|-------------|--------|
| `/bandwidth-profiles` | CRUD |
| `/subscriptions` | CRUD + activate/suspend/isolate/restore/terminate |
| `/hotspot-sales` | List |
| `/ppp/profiles`, `/ppp/secrets`, `/ppp/active` | CRUD + WebSocket listen |
| `/hotspot/profiles`, `/hotspot/users`, `/hotspot/active`, `/hotspot/hosts`, `/hotspot/servers` | CRUD + WebSocket |
| `/queue/simple` | List |
| `/firewall/filter`, `/firewall/nat`, `/firewall/address-list` | List |
| `/ip/pools`, `/ip/addresses` | CRUD / List |
| `/monitor/system`, `/monitor/interfaces` | Get + WebSocket (traffic, logs, ping) |
| `/raw` | Run command + WebSocket listen |
| `/mikhmon/vouchers`, `/mikhmon/profiles`, `/mikhmon/reports`, `/mikhmon/expire` | Full Mikhmon management |

**Billing:**

| Method | Path | Handler |
|--------|------|---------|
| GET | `/invoices` | BillingHandler.ListInvoices |
| GET | `/invoices/overdue` | BillingHandler.GetOverdue |
| GET/DELETE | `/invoices/:id` | BillingHandler.GetInvoice/CancelInvoice |
| POST | `/invoices/trigger-monthly` | BillingHandler.TriggerMonthlyBilling |
| GET/POST | `/payments` | PaymentHandler.List/Create |
| GET | `/payments/:id` | PaymentHandler.Get |
| POST | `/payments/:id/confirm` | PaymentHandler.Confirm |
| POST | `/payments/:id/reject` | PaymentHandler.Reject |
| POST | `/payments/:id/refund` | PaymentHandler.Refund |
| POST | `/payments/:id/initiate-gateway` | PaymentHandler.InitiateGateway |

**Sales Agents:**

| Method | Path | Handler |
|--------|------|---------|
| GET/POST | `/sales-agents` | SalesAgentHandler.List/Create |
| GET/PUT/DELETE | `/sales-agents/:id` | Get/Update/Delete |
| GET/POST | `/sales-agents/:id/profile-prices` | List/UpsertProfilePrice |

**Agent Invoices:**

| Method | Path | Handler |
|--------|------|---------|
| GET | `/agent-invoices` | AgentInvoiceHandler.List |
| GET | `/agent-invoices/:id` | Get |
| POST | `/agent-invoices/:id/pay` | MarkPaid |
| POST | `/agent-invoices/:id/cancel` | Cancel |
| POST | `/agent-invoices/process-scheduled` | ProcessScheduled |

**Registrations:**

| Method | Path | Handler |
|--------|------|---------|
| GET | `/registrations` | RegistrationHandler.List |
| GET | `/registrations/:id` | Get |
| POST | `/registrations/:id/approve` | Approve |
| POST | `/registrations/:id/reject` | Reject |

**Cash Management:**

| Method | Path | Handler |
|--------|------|---------|
| GET/POST | `/cash-entries` | CashManagementHandler.ListEntries/CreateEntry |
| GET/PUT/DELETE | `/cash-entries/:id` | Get/Update/Delete |
| POST | `/cash-entries/:id/approve` | ApproveEntry |
| POST | `/cash-entries/:id/reject` | RejectEntry |
| GET/POST | `/petty-cash-funds` | ListFunds/CreateFund |
| GET/PUT | `/petty-cash-funds/:id` | GetFund/UpdateFund |
| POST | `/petty-cash-funds/:id/topup` | TopUpFund |
| GET | `/reports/summary` | ReportHandler.GetSummary |
| GET | `/reports/subscriptions` | GetSubscriptions |
| GET | `/reports/cash-flow` | GetCashFlow |
| GET | `/reports/cash-balance` | GetCashBalance |
| GET | `/reports/reconciliation` | GetReconciliation |
| GET/POST | `/settings` | SystemSettingHandler.List/Upsert |

### 9.4 Customer Portal Routes (`/portal/v1`)

| Method | Path | Handler |
|--------|------|---------|
| GET | `/profile` | GetProfile |
| PUT | `/profile/password` | ChangePortalPassword |
| GET | `/subscriptions` | GetSubscriptions |
| GET | `/invoices`, `/invoices/:id` | GetInvoices/GetInvoice |
| GET/POST | `/payments`, `/payments/:id` | GetPayments/CreatePayment/GetPayment |
| POST | `/payments/:id/pay` | PayWithGateway |

### 9.5 Agent Portal Routes (`/agent-portal/v1`)

| Method | Path | Handler |
|--------|------|---------|
| GET | `/profile` | GetProfile |
| PUT | `/profile/password` | ChangePassword |
| GET | `/invoices`, `/invoices/:id` | GetInvoices/GetInvoice |
| POST | `/invoices/:id/request-payment` | RequestPayment |
| GET | `/sales` | GetSales |

---

## 10. Security

### 10.1 Authentication

| Mechanism | Implementation |
|-----------|---------------|
| Token Type | JWT (HS256) |
| Token Generation | `pkg/jwt/jwt.go` -- `golang-jwt/jwt/v5` |
| Access Token Expiry | Configurable (default 1h) |
| Refresh Token Expiry | Configurable (default 168h / 7 days) |
| Token Types | access, refresh, portal, agent_portal |
| Session Store | Redis (`session:{token}`) |
| Token Blacklist | Redis (`blacklist:{jti}`) |
| Password Change Invalidation | Redis (`pwd_changed:{userID}`) -- forces re-auth after password change |
| Password Hashing | bcrypt (via `golang.org/x/crypto`) |

### 10.2 Authorization (RBAC)

| Component | Implementation |
|-----------|---------------|
| Policy Engine | Casbin v3 with GORM adapter |
| Model | RBAC + regex matching (`model.conf`) |
| Policy Storage | `casbin_rule` PostgreSQL table |
| Enforcement | `CasbinMiddleware` extracts role from JWT claims |
| Roles | superadmin, admin, cs, billing, technician, readonly |
| Role Inheritance | superadmin->admin, cs/billing/technician->staff |
| Admin Full Access | `admin` -> `/api/v1/*` -> `.*` |
| Staff Limited Access | Specific paths, GET/POST/PUT/DELETE |

### 10.3 Network Security

| Measure | Implementation |
|---------|---------------|
| CORS | Configurable origins via `AllowedOrigins` env var |
| Rate Limiting | Redis sliding window (`pkg/redis/ratelimit.go`) |
| Request ID | `gin-contrib/requestid` |
| WebSocket Origin | Configurable allowlist (`pkg/ws/upgrader.go`) |
| Router Password | AES-GCM encryption in database (`utils/encrypt.go`) |

### 10.4 Data Security

| Measure | Implementation |
|---------|---------------|
| SQL Injection | GORM parameterized queries |
| Password Storage | bcrypt hashing |
| Router Credentials | AES-GCM encryption at rest |
| Audit Trail | `audit_logs` table with old/new values (jsonb) |
| System Settings | `IsEncrypted` flag for sensitive settings |

### 10.5 Payment Security

| Measure | Implementation |
|---------|---------------|
| Webhook Verification | Xendit: `x-callback-token` header validation |
| Payment Allocation | FIFO, transactional (`Transactor.RunInTx`) |
| Gateway Flow | Server-side invoice creation, redirect to payment URL |

### 10.6 Frontend Security

| Measure | Implementation |
|---------|---------------|
| Token Storage | localStorage (Zustand persist) |
| Silent Refresh | Admin client queues concurrent requests during refresh |
| 401 Handling | Auto-redirect to login for all 3 auth systems |
| Input Validation | Zod schemas on all forms |

---

## 11. Patterns and Conventions

### 11.1 Backend Patterns

| Pattern | Implementation |
|---------|---------------|
| **Registry** | Every layer has a `Registry` struct aggregating all instances, constructed in `cmd/server/main.go` |
| **Layered Architecture** | Router -> Handler -> Service -> Domain + Repository -> Model |
| **Domain-Driven Design** | 7 pure-logic domain modules with no external dependencies |
| **Repository Pattern** | 19 interfaces in `internal/repository/`, PostgreSQL implementations in `internal/repository/postgres/` |
| **Interface Segregation** | Small, focused interfaces (Repository pattern applied to MikroTik API too) |
| **Dependency Injection** | Constructor functions with explicit dependencies (no DI framework) |
| **Unit of Work** | `Transactor` interface for transactional payment processing |
| **Strategy** | `payment.Provider` interface for payment gateway abstraction |
| **Observer** | RabbitMQ producers/consumers for async billing/suspend/notification |
| **Connection Pool** | `pool.ConnPool` per pipeline type in collector |
| **Fan-In** | `ListenManyArgsContext` merges multiple RouterOS streams into single channel |
| **Batch Write** | `BatchWriter` with configurable size + flush interval |
| **Circuit Breaker** | Auto-reconnect with exponential backoff (1s-30s) in RouterOS client |
| **Graceful Shutdown** | 30s timeout on SIGINT/SIGTERM |
| **Idempotent Seeds** | `ON CONFLICT DO NOTHING` for all seed data |

### 11.2 Frontend Patterns

| Pattern | Implementation |
|---------|---------------|
| **File-Based Routing** | TanStack Router with auto-generated route tree |
| **Feature-Based Organization** | Each feature module has components, data (columns/schema), and index |
| **Custom Hooks per API** | Every API function wrapped in TanStack Query hooks |
| **Optimistic State** | TanStack Query mutations with automatic cache invalidation |
| **URL State Sync** | `useTableUrlState` hook syncs table pagination/filters to URL |
| **3-Client Architecture** | Separate Axios instances for admin/customer/agent with different auth flows |
| **Zustand for Auth** | Persisted state for tokens/user, avoids prop drilling |
| **Context for UI** | Theme/font/direction/layout preferences in React Context (cookie-persisted) |
| **Generic WebSocket** | `ForwardChannel[T]` for type-safe streaming of any data type |
| **Shadcn UI + Radix** | Unstyled headless components with Tailwind styling |
| **Zod Validation** | Shared schemas for form validation and API response typing |
| **Data Table Pattern** | Reusable DataTable components (pagination, column header, toolbar, bulk actions, faceted filter) |

### 11.3 Code Generation

The Makefile provides scaffolding targets that generate boilerplate:

```bash
make model VAL=plan              # internal/model/plan.go
make domain VAL=plan             # internal/domain/plan/domain.go
make service VAL=plan            # internal/service/plan_service.go
make handler VAL=plan            # internal/handler/plan_handler.go
make repository VAL=plan         # internal/repository/plan_repo.go + postgres/plan_repo.go
make migration VAL=add_plan      # internal/migration/0XX_add_plan.go
make mikrotik-domain VAL=wifi    # pkg/mikrotik/domain/wifi.go
make mikrotik-module VAL=wifi    # pkg/mikrotik/wifi/service.go + repository.go + test.go
```

### 11.4 Naming Conventions

| Layer | Convention | Example |
|-------|-----------|---------|
| Models | PascalCase struct, snake_case table | `BandwidthProfile` -> `bandwidth_profiles` |
| Repositories | `{Entity}Repository` interface | `CustomerRepository` |
| Services | `{Entity}Service` struct | `BillingService` |
| Handlers | `{Entity}Handler` struct | `PaymentHandler` |
| DTOs | `{Entity}Request`/`{Entity}Response` | `CreatePaymentRequest` |
| Files | snake_case, one primary type per file | `bandwidth_profile.go` |
| Migrations | `{NNN}_{description}.go` | `028_create_cash_management.go` |
| Frontend hooks | `use-{entity}-{action}.ts` | `use-payments.ts` |
| Frontend features | kebab-case directory | `bandwidth-profiles/` |

---

## 12. Strengths

### 12.1 Architecture

- **Clean separation of concerns**: Router -> Handler -> Service -> Domain + Repository -> Model layers with clear boundaries
- **Pure domain logic**: 7 domain modules with zero external dependencies, easily unit-testable
- **Registry pattern**: Explicit dependency wiring in a single file (`cmd/server/main.go`) makes the full dependency graph visible
- **Interface-driven design**: 19 repository interfaces + payment Provider interface enable easy mocking and testing

### 12.2 MikroTik Integration

- **Custom RouterOS client** (`pkg/mikrotik/`) replaces third-party library with purpose-built async client with auto-reconnect
- **3-Pipeline collector** separates time-series (InfluxDB), operational state (Redis), and on-demand data intelligently
- **Connection pooling** per pipeline type prevents resource exhaustion
- **Repository pattern on MikroTik API** makes RouterOS commands testable and interchangeable

### 12.3 ISP Domain Coverage

- **Complete subscription lifecycle**: 6-state machine (pending/active/suspended/isolated/expired/terminated) with validated transitions
- **Full billing pipeline**: Invoice generation, payment processing, FIFO allocation, overdue detection, automated isolation
- **Multi-portal architecture**: Admin, Customer, and Agent portals with separate auth systems
- **Sales agent ecosystem**: Agent management, profile pricing, voucher sales tracking, agent invoicing
- **Cash management**: Income/expense tracking, petty cash funds, reconciliation reports

### 12.4 Developer Experience

- **Comprehensive Makefile**: 30+ code generation targets reduce boilerplate to a single command
- **Auto-seeding**: Idempotent seed data for rapid development reset
- **Test infrastructure**: 29 HTTP test collections + 41 integration test files + mock generation
- **Frontend scaffolding**: Feature-based organization with consistent patterns (columns, schema, table, dialogs)

### 12.5 Observability

- **Structured logging**: Zap with environment-aware formatting
- **Audit trail**: Full old/new value tracking for admin actions
- **Time-series monitoring**: InfluxDB + embedded dashboard for real-time router metrics
- **Request tracing**: Request ID middleware

### 12.6 Frontend Quality

- **Type-safe routing**: TanStack Router with auto-generated route tree
- **Type-safe API**: Zod schemas + TanStack Query hooks for all endpoints
- **Silent token refresh**: Queued concurrent request pattern prevents duplicate API calls during refresh
- **Real-time WebSocket**: Generic `ForwardChannel[T]` pattern with `usePing` as reference implementation

---

## 13. Weaknesses and Improvement Areas

### 13.1 Testing Gaps

| Area | Issue |
|------|-------|
| **Unit test coverage** | No visible `*_test.go` files in `internal/service/`, `internal/domain/`, or `internal/handler/` -- domain layer is testable but appears untested |
| **Service layer tests** | 13 services with complex business logic (billing, payment allocation) lack unit tests |
| **Handler tests** | 32 HTTP handlers lack `httptest`-based handler tests |
| **Collector tests** | 3-pipeline collector system has no automated tests |
| **Frontend tests** | `vitest.config.ts` exists but no visible test files in feature modules |

**Recommendation**: Prioritize tests for `internal/domain/` (pure logic, easiest to test), then `internal/service/` with mocked repositories.

### 13.2 Security Concerns

| Area | Issue |
|------|-------|
| **JWT Secret** | `.env.example` has `your-secret-key-here` -- production may use weak secrets |
| **Development keys** | `.env.example` contains Xendit development keys (`xnd_development_...`) that should never be in version control |
| **GoWA credentials** | Hardcoded device ID and IP in `.env.example` |
| **Router passwords** | Seed data uses `r00t` as default password |
| **CSRF** | No CSRF protection visible for session-based flows |
| **Content Security Policy** | No CSP headers configured |
| **Token in localStorage** | Frontend stores JWT in localStorage (XSS-vulnerable); consider httpOnly cookies |
| **Test credentials** | Test MikroTik credentials hardcoded in `.env.example` |

**Recommendation**: Add `.env` to `.gitignore` verification, implement CSP headers, consider httpOnly cookie-based token storage.

### 13.3 Architectural Concerns

| Area | Issue |
|------|-------|
| **God main.go** | `cmd/server/main.go` manually wires 20+ layers (297 lines) -- consider wire/dig DI or at least分层 initialization |
| **No graceful degradation** | RabbitMQ failure is non-fatal but no retry/reconnection logic visible |
| **Duplicate collector** | `internal/collector/` (19 files) and `pkg/mikrotik/collector/` both implement collection -- unclear which is authoritative |
| **Payment gateway stubs** | Tripay and Midtrans clients return `ErrNotImplemented` -- should be removed or feature-flagged |
| **Hardcoded config** | `cmd/mikrotik/main.go` has hardcoded router address, Redis port, InfluxDB URL |
| **No API versioning strategy** | Only `/api/v1` -- no plan for v2 migration |
| **Missing OpenAPI validation** | 6507-line OpenAPI spec exists but no code generation or validation against it |
| **DTO explosion** | 16 DTO files but some overlap with model structs -- consider using models directly for simple CRUD |

### 13.4 Operational Concerns

| Area | Issue |
|------|-------|
| **No health check endpoints** | Only `/health` exists -- no readiness/liveness probes for dependencies |
| **No Prometheus metrics** | Custom metrics via InfluxDB but no standard Prometheus exposition |
| **No distributed tracing** | Request IDs exist but no OpenTelemetry/jaeger integration |
| **Commented-out Docker services** | `app` and `nginx` services commented out in docker-compose -- production Docker setup incomplete |
| **No CI/CD** | No `.github/workflows/` or equivalent CI configuration |
| **Dockerfile version mismatch** | `go.mod` says Go 1.25.5, Dockerfile uses `golang:1.24-alpine` |

### 13.5 Frontend Concerns

| Area | Issue |
|------|-------|
| **Large bundle** | No evidence of bundle analysis or tree-shaking verification |
| **No error boundaries** | Error pages exist but no React Error Boundary components visible |
| **No E2E tests** | No Playwright/Cypress configuration |
| **Clerk routes orphaned** | `clerk/` routes exist but Clerk is not a dependency -- dead code |
| **No i18n** | UI appears hardcoded to English/Indonesian mix |
| **Admin role check** | Frontend doesn't appear to enforce role-based UI visibility (no role-gated menus) |

### 13.6 Database Concerns

| Area | Issue |
|------|-------|
| **Missing indexes** | No explicit index definitions visible in migrations beyond unique constraints |
| **JSONB queries** | `Tags` (jsonb) on Customer and `OldValue`/`NewValue` (jsonb) on AuditLog need GIN indexes |
| **No database backups** | No backup/restore strategy in docker-compose |
| **Migration gaps** | Migration numbers skip (012-014, 016 missing) -- suggests deleted migrations |
| **No soft delete** | Hard deletes on most entities -- audit trail relies on AuditLog table only |

---

## 14. Data Flow Diagrams

### 14.1 Subscription Lifecycle

```
+----------+     +--------+     +---------+     +----------+     +--------+
| pending  +---->| active +---->|suspended+---->| isolated +---->|expired |
+----+-----+     +---+----+     +----+----+     +----+-----+     +---+----+
     |               |               |               |              |
     |               |               +-------+       |              |
     |               |               |restore|<------+              |
     |               |               +---+---+                     |
     |               |                   |                         |
     v               v                   v                         v
     +----------->+-------------------> terminated <----------------+
```

### 14.2 Billing and Payment Flow

```
+----------------+     +----------------+     +---------------+
| Scheduler      |     | BillingService |     | InvoiceRepo   |
| (daily 00:00)  +---->| ProcessDaily   +---->| Create Invoice|
+----------------+     | Billing        |     +-------+-------+
                       +-------+--------+             |
                               |                      v
                               |              +-------+--------+
                               |              | Customer       |
                               |              | Notification   |
                               |              | (WhatsApp)     |
                               |              +----------------+
                               |
                    +----------v-----------+
                    | PaymentService       |
                    | CreatePayment        |
                    | Confirm/Reject/Refund|
                    +----+----------+------+
                         |          |
              +----------v--+  +---v-----------+
              | PaymentRepo |  | PaymentAlloc  |
              |             |  | Repo (FIFO)   |
              +-------------+  +-------+-------+
                                       |
                              +--------v--------+
                              | InvoiceRepo     |
                              | UpdateStatus    |
                              | (paid/overdue)  |
                              +-----------------+
```

### 14.3 Authentication Flow (Admin)

```
+-----------+     +------------+     +--------+     +--------+     +--------+
| Client    +---->| AuthHandler+---->| Auth   +---->| User   +---->| bcrypt |
| POST      |     | Login()    |     | Service|     | Repo   |     | verify |
| /login    |     +------+-----+     +---+----+     +--------+     +--------+
+-----------+            |               |
                         v               v
                  +------+------+  +-----+------+
                  | JWT Service  |  | Redis      |
                  | GeneratePair |  | SetSession |
                  +-------------+  +------------+
                         |
                  +------v------+
                  | Response    |
                  | {access,    |
                  |  refresh,   |
                  |  user}      |
                  +-------------+
                         |
            +------------v------------+
            | Subsequent requests     |
            | Authorization: Bearer   |
            +------------+------------+
                         |
                  +------v------+
                  | AuthMiddle  |
                  | ware        |
                  | JWT->Claims |
                  | Redis check |
                  | Role extract|
                  +------+------+
                         |
                  +------v------+
                  | CasbinMiddle|
                  | ware        |
                  | RBAC check  |
                  +------+------+
                         |
                  +------v------+
                  | Handler     |
                  +-------------+
```

### 14.4 MikroTik 3-Pipeline Collector

```
+------------------+
| MikroTik Router  |
| (RouterOS API)   |
+--------+---------+
         |
    +----v----+
    | Client  |  (pkg/mikrotik/client/)
    | Manager |  Multi-router connection pool
    +--+-+-+-+
      | | | |
      | | | +-----------------------------+
      | | |                               |
      | | +----------+                    |
      | v            v                    v
+-----+----+  +------+------+  +---------+------+
| Pipeline A|  | Pipeline B  |  | Pipeline C    |
| TimeSeries|  | Operational |  | OnDemand      |
+-----+-----+  +------+------+  +---------+------+
      |              |    |               |
      |         Tier 2   Tier 3          |
      |         (follow)  (ticker)        |
      v           v         v             v
+-----+-----+ +--+--+  +---+---+  +------+--+
| BatchWriter| |B.W. |  | B.W.  |  | Runner  |
| (influx)   | |(redis)| |(redis)|  |(direct) |
+-----+------+ +--+--+  +---+---+  +----+----+
      |           |          |            |
      v           v          v            v
+-----+----+ +----+---+ +---+----+ +-----+---+
| InfluxDB | | Redis  | | Redis  | | RouterOS|
| 3 Core   | | (state)| | (cache)| | API     |
+----------+ +--------+ +--------+ +---------+

Pipeline A specs: interface_traffic, queue_stats, system_resource
Pipeline B Tier2: ppp_active, hotspot_active, interface_status
Pipeline B Tier3: ppp_secrets(5m), ppp_profiles(5m), hotspot_users(5m), ip_pools(10m)
Pipeline C: on-demand read/write with Redis cache-through
```

### 14.5 Request Lifecycle (Admin API)

```
+----------+     +----------+     +-----------+     +----------+
| HTTP     +---->| CORS     +---->| RequestID +---->| Logger   |
| Request  |     |Middleware|     | Middleware|     |Middleware|
+----------+     +----------+     +-----------+     +----+-----+
                                                        |
                                                 +------v------+
                                                 | Auth        |
                                                 | Middleware  |
                                                 | (JWT+Redis) |
                                                 +------+------+
                                                        |
                                                 +------v------+
                                                 | Casbin      |
                                                 | Middleware  |
                                                 | (RBAC check)|
                                                 +------+------+
                                                        |
                                                 +------v------+
                                                 | Mikrotik    |
                                                 | Router      |
                                                 | Middleware  |
                                                 | (if needed) |
                                                 +------+------+
                                                        |
                                                 +------v------+
                                                 | Handler     |
                                                 | (Gin)       |
                                                 +------+------+
                                                        |
                                                 +------v------+
                                                 | Service     |
                                                 | (business)  |
                                                 +------+------+
                                                    |   |   |
                                              +-----+   |   +------+
                                              v         v          v
                                        +-----+---+ +---+----+ +--+------+
                                        | Domain  | | Repo   | | Queue   |
                                        | (logic) | | (data) | | (async) |
                                        +---------+ +---+----+ +---------+
                                                        |
                                                  +-----v------+
                                                  | PostgreSQL |
                                                  +------------+
```

### 14.6 Frontend Auth Flow

```
+----------------+     +------------+     +-------------+
| Login Form     +---->| useAdmin   +---->| adminLogin()|
| (email+pass)   |     | Login()    |     | POST /login |
+----------------+     +-----+------+     +------+------+
                             |                   |
                    +--------v-------+   +-------v--------+
                    | useAuthStore   |   | Response       |
                    | adminSetTokens |   | {access,       |
                    | adminSetUser   |   |  refresh, user}|
                    +--------+-------+   +----------------+
                             |
                    +--------v-------+
                    | adminClient    |
                    | Bearer token   |
                    | interceptor    |
                    +--------+-------+
                             |
                    +--------v-------+
                    | TanStack Query |
                    | hooks          |
                    | (useCustomers, |
                    |  usePayments)  |
                    +--------+-------+
                             |
                    +--------v-------+
                    | API response   |
                    | auto-invalidate|
                    +--------+-------+
                             |
                    +--------v-------+  401
                    | Silent refresh +-------> adminRefreshToken()
                    | (queued reqs)  |        POST /auth/refresh
                    +----------------+             |
                                            +-----v------+
                                            | If refresh |
                                            | fails:     |
                                            | clear auth |
                                            | redirect   |
                                            | /sign-in   |
                                            +------------+
```

---

## Appendix A: Key File Counts

| Category | Count |
|----------|-------|
| Go entry points (`cmd/*/main.go`) | 5 |
| GORM models | 21 |
| Domain modules | 7 |
| Service structs (core + mikrotik + mikhmon) | 27 |
| Repository interfaces | 19 |
| Handler structs | 32 |
| Middleware | 9 |
| Route files | 10 |
| Migration files | 25 |
| Seeder files | 8 |
| Queue producers/consumers | 6 |
| Scheduler jobs | 6 |
| pkg/ packages | 12 |
| pkg/mikrotik/ files | 90+ |
| Frontend feature modules | 15 |
| Frontend API modules | 18+ |
| Frontend custom hooks | 20 |
| Frontend route files | 42 |
| Frontend shared components | 65 |
| HTTP test collections | 29 |
| Integration test files | 41 |

## Appendix B: Dependency Graph (Go)

```
mikmongo (go 1.25.5)
  |
  +-- github.com/gin-gonic/gin v1.12
  +-- gorm.io/gorm v1.31 + gorm.io/driver/postgres
  +-- github.com/jackc/pgx/v5
  +-- github.com/redis/go-redis/v9
  +-- github.com/rabbitmq/amqp091-go
  +-- github.com/InfluxCommunity/influxdb3-go/v2
  +-- github.com/golang-jwt/jwt/v5
  +-- github.com/casbin/casbin/v3 + gorm-adapter/v3
  +-- github.com/xendit/xendit-go/v7
  +-- github.com/pressly/goose/v3
  +-- github.com/robfig/cron/v3
  +-- github.com/go-playground/validator/v10
  +-- github.com/gorilla/websocket v1.5
  +-- go.uber.org/zap v1.27
  +-- github.com/joho/godotenv
  +-- github.com/google/uuid
  +-- github.com/gin-contrib/cors + requestid
  +-- golang.org/x/crypto
  +-- github.com/stretchr/testify
  +-- github.com/Butterfly-Student/go-ros (local replace -> ./pkg/mikrotik)
```
