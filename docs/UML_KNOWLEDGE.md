# MikMongo — UML Knowledge Base

> **Purpose:** Complete reference knowledge for generating UML diagrams (Class, ER, Sequence, Use Case, Component, Deployment, State) of the MikMongo ISP management platform.
>
> **Project:** mikmongo — ISP Customer/Billing/MikroTik management platform
> **Stack:** Go (Gin), PostgreSQL (GORM), Redis, Casbin RBAC, MikroTik RouterOS API, InfluxDB v3 (collector), JWT auth
> **Architecture:** Layered (Handler → Service → Domain → Repository → Model) with Domain-Driven boundaries
> **Generated:** 2026-04-18

---

## 1. High-Level Architecture

### 1.1 Layer Structure

```
┌──────────────────────────────────────────────────────────────────┐
│  HTTP Clients (Admin UI / Customer Portal / Agent Portal / WebUI) │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Router Layer  (Gin)                                              │
│  - public.go       (health, login, register, webhooks, portal)    │
│  - admin.go        (JWT + Casbin RBAC protected)                  │
│  - agent_portal.go (Agent JWT protected)                          │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Middleware Layer                                                 │
│  CORS · Logger · RequestID · Auth(JWT) · RBAC(Casbin)             │
│  PortalAuth · AgentPortalAuth · MikrotikRouter · RateLimit        │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Handler Layer  (18 HTTP handlers)                                │
│  Auth · User · Customer · Router · BandwidthProfile ·             │
│  Subscription · Billing · Payment · Registration ·                │
│  Webhook · SystemSetting · CustomerPortal · Report ·              │
│  HotspotSale · SalesAgent · AgentInvoice · AgentPortal ·          │
│  CashManagement · Mikrotik(PPP/Hotspot/IP/Queue/Monitor/Raw)      │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Service Layer  (12 services)                                     │
│  Auth · Customer · BandwidthProfile · Billing · Payment ·         │
│  Subscription · Registration · Router · Notification · Report ·   │
│  AgentInvoice · CashManagement                                    │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Domain Layer  (7 domains - pure business rules, no IO)           │
│  Customer · Billing · Payment · Router · Subscription ·           │
│  Registration · Notification                                      │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Repository Layer  (20 repos + Transactor)                        │
│  GORM-based CRUD · Transactor for multi-model operations          │
└──────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│  Infrastructure                                                   │
│  PostgreSQL · Redis (cache) · Casbin(RBAC) · JWT ·                │
│  MikroTik RouterOS API · WhatsApp(GOWA) · InfluxDB v3             │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 Entry Points (`cmd/`)

| Binary | Purpose |
|---|---|
| `cmd/mikrotik/main.go` | Main HTTP server (Gin) |
| `cmd/migrate/main.go` | Goose migrations runner |
| `cmd/seed/main.go` | Seeds users, casbin rules, system settings |
| `cmd/full_test/main.go` | Full integration test driver |

---

## 2. Domain Model (Class / ER Diagram Source)

> All entities use **UUID** primary keys (PostgreSQL `gen_random_uuid()`).
> All mutable entities use GORM soft-delete via `DeletedAt`.
> All timestamps are `timestamptz`.

### 2.1 Entity Catalog (22 Models)

| # | Entity | Table | Purpose |
|---|---|---|---|
| 1  | User                    | `users`                   | Admin/operator/technician/sales_agent/customer auth |
| 2  | Customer                | `customers`               | ISP end customer |
| 3  | MikrotikRouter          | `mikrotik_routers`        | Managed RouterOS devices |
| 4  | BandwidthProfile        | `bandwidth_profiles`      | Service plans/packages (router-scoped) |
| 5  | Subscription            | `subscriptions`           | Customer service binding (customer × plan × router) |
| 6  | Invoice                 | `invoices`                | Customer billing document |
| 7  | InvoiceItem             | `invoice_items`           | Invoice line items |
| 8  | Payment                 | `payments`                | Customer payments |
| 9  | PaymentAllocation       | `payment_allocations`     | Many-to-many between Payment & Invoice |
| 10 | CustomerRegistration    | `customer_registrations`  | Self-service signup requests |
| 11 | HotspotSale             | `hotspot_sales`           | Hotspot voucher sale records |
| 12 | SalesAgent              | `sales_agents`            | Hotspot voucher resellers |
| 13 | SalesProfilePrice       | `sales_profile_prices`    | Per-agent voucher pricing |
| 14 | AgentInvoice            | `agent_invoices`          | Periodic agent billing statement |
| 15 | CashEntry               | `cash_entries`            | Cash book income/expense |
| 16 | PettyCashFund           | `petty_cash_funds`        | Petty cash float |
| 17 | MessageTemplate         | `message_templates`       | WhatsApp/Email templates |
| 18 | SystemSetting           | `system_settings`         | Key/value config store |
| 19 | AuditLog                | `audit_logs`              | Change audit trail |
| 20 | SequenceCounter         | `sequence_counters`       | Document numbering (INV-, PAY-, etc.) |
| 21 | CasbinRule              | `casbin_rule`             | RBAC policies (Casbin managed) |
| 22 | BandwidthProfileConfig  | *(embedded)*              | PPPoE JSON helper (not persisted) |

### 2.2 Entity Details

#### User — `users`
- `id` UUID PK
- `full_name` varchar(100) NOT NULL
- `email` varchar(100) UNIQUE NOT NULL
- `phone` varchar(20)
- `password_hash` varchar(255) NOT NULL
- `role` varchar(20) NOT NULL DEFAULT `'customer'`
  - **CHECK** IN (`superadmin`, `admin`, `technician`, `sales_agent`, `customer`)
- `is_active` bool DEFAULT true
- `last_login` timestamptz NULL
- `last_ip` varchar(45)
- `bearer_key` varchar(255) UNIQUE NULL
- `created_at`, `updated_at`, `deleted_at`

#### Customer — `customers`
- `id` UUID PK
- `customer_code` varchar(50) UNIQUE NOT NULL (e.g. `CUST-00001`)
- `full_name`, `email` (UNIQUE), `username` (UNIQUE), `phone` NOT NULL
- `id_card_number`, `address`, `latitude` dec(10,8), `longitude` dec(11,8)
- `is_active` bool
- `portal_password_hash` (never in JSON)
- `portal_last_login`
- `notes` text, `tags` jsonb
- `created_by` → `users.id`, `updated_by` → `users.id`

#### MikrotikRouter — `mikrotik_routers`
- `id` UUID PK
- `name`, `address` varchar(100) NOT NULL
- `area` varchar(100) NULL
- `api_port` int DEFAULT 8728
- `rest_port` int DEFAULT 80
- `username` varchar(100) NOT NULL
- `password_encrypted` text (AES)
- `use_ssl`, `is_master`, `is_active` bool
- `status` varchar(20) DEFAULT `'unknown'` CHECK IN (`online`,`offline`,`unknown`)
- `last_seen_at` timestamptz NULL
- `notes` text

#### BandwidthProfile — `bandwidth_profiles`
- `id` UUID PK
- `router_id` UUID NOT NULL → `mikrotik_routers.id`
- `profile_code` varchar(50), `name`, `description`
- `download_speed`, `upload_speed` bigint (bps)
- `price_monthly` dec(12,2)
- `tax_rate` dec(5,4) DEFAULT 0.11
- `billing_cycle` CHECK IN (`daily`,`weekly`,`monthly`,`yearly`)
- `billing_day` int, `grace_period_days` int DEFAULT 3
- `is_active`, `is_visible`, `sort_order`
- `isolate_profile_name`, `rate_limit` (e.g. `10M/10M`)

#### Subscription — `subscriptions`
- `id` UUID PK
- `customer_id` UUID NOT NULL → `customers.id`
- `plan_id` UUID NOT NULL → `bandwidth_profiles.id`
- `router_id` UUID NOT NULL → `mikrotik_routers.id`
- `previous_plan_id` UUID NULL → `bandwidth_profiles.id`
- `username` varchar(100) UNIQUE NOT NULL
- `password` (never in JSON)
- `static_ip` varchar(45), `gateway` varchar(15)
- `mt_ppp_id` varchar(50) (MikroTik PPP secret ID)
- `status` CHECK IN (`pending`,`active`,`suspended`,`isolated`,`expired`,`terminated`)
- `activated_at`, `expiry_date`, `terminated_at`, `billing_day`
- `auto_isolate` bool DEFAULT true
- `grace_period_days`, `suspend_reason`
- `created_by` → `users.id`

#### Invoice — `invoices`
- `id` UUID PK
- `invoice_number` varchar(50) UNIQUE (e.g. `INV-202604-00001`)
- `customer_id` UUID → `customers.id`
- `subscription_id` UUID NULL → `subscriptions.id`
- `billing_period_start`, `billing_period_end` date
- `billing_month` (1-12), `billing_year`
- `issue_date`, `due_date`, `payment_deadline`
- `subtotal`, `tax_amount`, `discount_amount`, `late_fee`, `total_amount` dec(12,2)
- `paid_amount`, `balance` (computed column)
- `status` CHECK IN (`draft`,`sent`,`unpaid`,`partial`,`paid`,`overpaid`,`overdue`,`cancelled`,`refunded`)
- `payment_date`, `payment_method`
- `invoice_type` CHECK IN (`recurring`,`installation`,`additional`,`refund`)
- `is_auto_generated`, `reminder_sent_count`, `last_reminder_sent`
- `notes`, `internal_notes`
- `created_by`, `updated_by` → `users.id`

#### InvoiceItem — `invoice_items`
- `id` UUID PK
- `invoice_id` UUID NOT NULL → `invoices.id`
- `item_type` CHECK IN (`subscription`,`installation`,`equipment`,`other`)
- `description` varchar(255)
- `profile_id` UUID NULL → `bandwidth_profiles.id`
- `quantity`, `unit_price`, `subtotal`, `tax_rate`, `tax_amount`, `total`
- `is_prorated`, `proration_days`, `proration_percentage`
- `sort_order`

#### Payment — `payments`
- `id` UUID PK
- `payment_number` varchar(50) UNIQUE (e.g. `PAY-202604-00001`)
- `customer_id` UUID → `customers.id`
- `amount`, `allocated_amount`, `remaining_amount` (computed)
- `payment_method` CHECK IN (`cash`,`bank_transfer`,`e-wallet`,`credit_card`,`debit_card`,`check`,`qris`,`gateway`)
- `payment_date` timestamptz NOT NULL
- `bank_name`, `bank_account_number`, `bank_account_name`
- `transaction_reference`
- `ewallet_provider`, `ewallet_number`
- `gateway_name`, `gateway_trx_id`, `gateway_response` jsonb, `gateway_payment_url`
- `proof_image` text (base64), `receipt_number`
- `status` CHECK IN (`pending`,`confirmed`,`rejected`,`refunded`)
- `processed_by`, `refunded_by`, `created_by` → `users.id`
- `processed_at`, `rejection_reason`
- `refund_amount`, `refund_date`, `refund_reason`

#### PaymentAllocation — `payment_allocations`
- `id` UUID PK
- `payment_id` UUID NOT NULL → `payments.id`
- `invoice_id` UUID NOT NULL → `invoices.id`
- `allocated_amount` dec(12,2)

#### CustomerRegistration — `customer_registrations`
- `id` UUID PK
- `full_name`, `phone` NOT NULL, `email`, `address`, `latitude`, `longitude`
- `bandwidth_profile_id` UUID NULL → `bandwidth_profiles.id`
- `status` CHECK IN (`pending`,`approved`,`rejected`) DEFAULT `'pending'`
- `rejection_reason`
- `approved_by` → `users.id`, `approved_at`
- `customer_id` → `customers.id` (set on approval)

#### HotspotSale — `hotspot_sales`
- `id` UUID PK
- `router_id` UUID → `mikrotik_routers.id`
- `username`, `profile` varchar(100)
- `price`, `selling_price` dec(15,2)
- `prefix`, `batch_code`
- `sales_agent_id` UUID NULL → `sales_agents.id`

#### SalesAgent — `sales_agents`
- `id` UUID PK
- `router_id` UUID → `mikrotik_routers.id`
- `name`, `phone`
- `username` varchar(50) UNIQUE, `password_hash` (hidden)
- `status` CHECK IN (`active`,`inactive`)
- `voucher_mode` CHECK IN (`mix`,`num`,`alp`)
- `voucher_length` int DEFAULT 6
- `voucher_type` CHECK IN (`upp`,`up`)
- `bill_discount` dec(15,2)
- `billing_cycle` CHECK IN (`weekly`,`monthly`)
- `billing_day` int

#### SalesProfilePrice — `sales_profile_prices`
- `id` UUID PK
- `sales_agent_id` UUID → `sales_agents.id`
- `profile_name` varchar(100)
- `base_price`, `selling_price` dec(15,2)
- `voucher_length`, `is_active`

#### AgentInvoice — `agent_invoices`
- `id` UUID PK
- `agent_id` UUID → `sales_agents.id`
- `router_id` UUID → `mikrotik_routers.id`
- `invoice_number` varchar(20) UNIQUE
- `billing_cycle` (`weekly` | `monthly`)
- `period_start`, `period_end`
- `billing_month`, `billing_week` (ISO), `billing_year`
- `voucher_count` int
- `subtotal`, `selling_total`, `profit` (computed), `discount_amount`, `total_amount`, `paid_amount`, `balance` (computed)
- `status` (`draft` | `unpaid` | `paid` | `cancelled`)

#### CashEntry — `cash_entries`
- `id` UUID PK
- `entry_number` UNIQUE
- `type` (`income` | `expense`)
- `source` (`invoice` | `agent_invoice` | `installation` | `penalty` | `other` | `operational` | `upstream` | `purchase` | `salary`)
- `amount`, `description`
- `reference_type`, `reference_id` (polymorphic FK)
- `payment_method`, `bank_name`, `account_number`
- `petty_cash_fund_id` UUID NULL → `petty_cash_funds.id`
- `entry_date`
- `created_by`, `approved_by` → `users.id`
- `status` (pending/approved/rejected), `receipt_image`

#### PettyCashFund — `petty_cash_funds`
- `id`, `fund_name`
- `initial_balance`, `current_balance` dec(15,2)
- `custodian_id` UUID → `users.id`
- `status`

#### MessageTemplate — `message_templates`
- `id`, `event` varchar(80), `channel` (`whatsapp`|`email`)
- **UNIQUE** (event, channel)
- `subject` (email only), `body`, `is_active`

#### SystemSetting — `system_settings`
- `id`, `group_name`, `key_name`
- **UNIQUE** (group_name, key_name)
- `value` text, `type` (`string`|`integer`|`boolean`|`json`|`password`)
- `label`, `description`
- `is_encrypted`, `is_public`
- `updated_by` → `users.id`

#### AuditLog — `audit_logs`
- `id`, `admin_id` → `users.id`
- `action` varchar(100), `entity_type`, `entity_id`
- `old_value`, `new_value` jsonb
- `ip_address`, `user_agent`, `notes`

#### SequenceCounter — `sequence_counters`
- `id`, `name` UNIQUE
- `prefix`, `padding`, `last_number`
- `reset_monthly`, `reset_yearly`, `last_reset`
- *Thread-safe via `SELECT FOR UPDATE`*

### 2.3 Relationship Matrix (ER Edges)

```
User (1) ──┬── (N) Customer         [created_by, updated_by]
           ├── (N) Subscription     [created_by]
           ├── (N) Invoice          [created_by, updated_by]
           ├── (N) Payment          [processed_by, refunded_by, created_by]
           ├── (N) CustomerRegistration [approved_by]
           ├── (N) SystemSetting    [updated_by]
           ├── (N) PettyCashFund    [custodian_id]
           ├── (N) CashEntry        [created_by, approved_by]
           └── (N) AuditLog         [admin_id]

Customer (1) ──┬── (N) Subscription
               ├── (N) Invoice
               ├── (N) Payment
               └── (N) CustomerRegistration [on approval]

MikrotikRouter (1) ──┬── (N) BandwidthProfile
                     ├── (N) Subscription
                     ├── (N) SalesAgent
                     └── (N) HotspotSale

BandwidthProfile (1) ──┬── (N) Subscription  [as plan_id]
                       ├── (N) Subscription  [as previous_plan_id]
                       ├── (N) InvoiceItem   [profile_id]
                       └── (N) CustomerRegistration

Subscription (1) ── (N) Invoice

Invoice (1) ──┬── (N) InvoiceItem
              └── (N) PaymentAllocation

Payment (1) ── (N) PaymentAllocation  ── (N,1) Invoice
   (M:N between Payment ↔ Invoice via PaymentAllocation)

SalesAgent (1) ──┬── (N) SalesProfilePrice
                 ├── (N) HotspotSale
                 └── (N) AgentInvoice

PettyCashFund (1) ── (N) CashEntry

CustomerRegistration ── (0..1) Customer  [materialized post-approval]
CustomerRegistration ── (0..1) BandwidthProfile
```

### 2.4 Enum Reference (for State Machines / UML notes)

| Field | Values |
|---|---|
| `User.role` | superadmin, admin, technician, sales_agent, customer |
| `Subscription.status` | pending → active → (suspended \| isolated \| expired \| terminated) |
| `Invoice.status` | draft, sent, unpaid, partial, paid, overpaid, overdue, cancelled, refunded |
| `Invoice.invoice_type` | recurring, installation, additional, refund |
| `Payment.status` | pending → (confirmed \| rejected) → refunded |
| `Payment.payment_method` | cash, bank_transfer, e-wallet, credit_card, debit_card, check, qris, gateway |
| `MikrotikRouter.status` | online, offline, unknown |
| `BandwidthProfile.billing_cycle` | daily, weekly, monthly, yearly |
| `SalesAgent.voucher_mode` | mix, num, alp |
| `SalesAgent.voucher_type` | upp, up |
| `AgentInvoice.status` | draft, unpaid, paid, cancelled |
| `CashEntry.type` | income, expense |
| `CashEntry.status` | pending, approved, rejected |
| `CustomerRegistration.status` | pending, approved, rejected |
| `MessageTemplate.channel` | whatsapp, email |
| `SystemSetting.type` | string, integer, boolean, json, password |

---

## 3. Layer Components (for Component/Class Diagrams)

### 3.1 Repository Layer (20 + Transactor)

`CustomerRepository · InvoiceRepository · PaymentRepository · RouterDeviceRepository · UserRepository · BandwidthProfileRepository · SubscriptionRepository · CustomerRegistrationRepository · InvoiceItemRepository · PaymentAllocationRepository · SystemSettingRepository · SequenceCounterRepository · MessageTemplateRepository · AuditLogRepository · HotspotSaleRepository · SalesAgentRepository · AgentInvoiceRepository · CashEntryRepository · PettyCashFundRepository · Transactor`

- Interfaces live in `internal/repository/interfaces.go`
- Concrete GORM impls one file each (`*_repo.go`)
- `Transactor` wraps multi-model tx using `gorm.DB.Transaction()`

### 3.2 Service Layer (12)

| Service | Depends on (Repos / Services / Infra) |
|---|---|
| **AuthService** | UserRepo, JWT, Redis |
| **CustomerService** | CustomerRepo, SequenceCounterRepo, BandwidthProfileRepo, CustomerDomain, RouterService, (SubscriptionService — injected) |
| **BandwidthProfileService** | BandwidthProfileRepo, RouterService, Redis |
| **BillingService** | InvoiceRepo, InvoiceItemRepo, SubscriptionRepo, BandwidthProfileRepo, CustomerRepo, SystemSettingRepo, SequenceCounterRepo, BillingDomain, (NotificationService, SubscriptionService — injected) |
| **PaymentService** | PaymentRepo, InvoiceRepo, PaymentAllocationRepo, CustomerRepo, SequenceCounterRepo, PaymentDomain, BillingDomain, Transactor, (CustomerService, NotificationService, CashManagementService — injected) |
| **SubscriptionService** | SubscriptionRepo, BandwidthProfileRepo, SystemSettingRepo, SubscriptionDomain, RouterService, Redis |
| **RegistrationService** | CustomerRegistrationRepo, CustomerService, SubscriptionService, (NotificationService — injected) |
| **RouterService** | RouterDeviceRepo, encKey (AES), Redis, Logger |
| **NotificationService** | MessageTemplateRepo, SystemSettingRepo, GOWA WhatsApp client |
| **ReportService** | *gorm.DB (raw queries) |
| **AgentInvoiceService** | AgentInvoiceRepo, HotspotSaleRepo, SalesAgentRepo, SequenceCounterRepo, (CashManagementService — injected) |
| **CashManagementService** | CashEntryRepo, PettyCashFundRepo, SequenceCounterRepo, *gorm.DB |

**Cross-service injections** (post-construction setters):
- `CustomerService.SetSubscriptionService`
- `BillingService.SetNotificationService` + `SetSubscriptionService`
- `PaymentService.SetCustomerService` + `SetNotificationService` + `SetCashManagementService`
- `RegistrationService.SetNotificationService`
- `AgentInvoiceService.SetCashManagementService`

### 3.3 Domain Layer (7)

Pure business logic, no IO. One package each:
- `customer.Domain` — customer validation, activation rules
- `billing.Domain` — invoice generation math (pro-rata, tax, late fee)
- `payment.Domain` — allocation rules, refund rules
- `router.Domain` — router integration invariants
- `subscription.Domain` — state machine transitions
- `registration.Domain` — approval workflow
- `notification.Domain` — template resolution rules

### 3.4 Handler Layer (18 + Mikrotik sub-registry)

```
Auth · User · Customer · BandwidthProfile · Subscription ·
Billing · Payment · Router · Registration · Webhook ·
SystemSetting · CustomerPortal · Report · HotspotSale ·
SalesAgent · AgentInvoice · AgentPortal · CashManagement
```

MikroTik sub-handlers: `Firewall · Hotspot (+ WS) · IP · PPP (+ WS) · Queue · Monitor (+ WS) · Raw · Mikhmon(Expire/Profile/Report/Voucher)`

### 3.5 Middleware Layer (9)

| Middleware | Applied to |
|---|---|
| `CORS` | all |
| `Logger` | all |
| `RequestID` | all |
| `AuthMiddleware` (JWT) | `/api/v1/*` except `/auth/login`, `/auth/refresh`, `/register`, `/webhooks/*` |
| `RBAC` (Casbin) | `/api/v1/*` (after Auth) |
| `PortalAuth` | `/portal/v1/*` (except login) |
| `AgentPortalAuth` | `/agent-portal/v1/*` (except login) |
| `MikrotikRouter` | router-scoped handlers (injects resolved router into ctx) |
| `RateLimit` | configurable per route |

### 3.6 RBAC (Casbin)

**Model**: RBAC with role inheritance (`g`) and regex path matching.

**Role hierarchy seeded at startup:**
```
superadmin ──► admin
technician ──► staff
sales_agent ──► staff
customer     (no inheritance — uses portal auth, not RBAC)
```

**Policies:**
- `admin → /api/v1/* → .*` (full access)
- `staff → /api/v1/auth/* → GET|POST`
- `staff → /api/v1/invoices[/*] → GET|POST|PUT|DELETE`
- `staff → /api/v1/payments[/*] → GET|POST|PUT`
- `staff → /api/v1/customers[/*] → GET`
- `staff → /api/v1/registrations[/*] → GET|POST|PUT`
- `staff → /api/v1/reports/* → GET`

---

## 4. Routes Map (for Sequence / Use Case Diagrams)

### 4.1 Public Routes (no auth)

| Method | Path | Handler |
|---|---|---|
| GET    | `/health` | inline |
| POST   | `/api/v1/auth/login` | Auth.Login |
| POST   | `/api/v1/auth/refresh` | Auth.RefreshToken |
| POST   | `/api/v1/register` | Registration.Create |
| POST   | `/api/v1/webhooks/midtrans` | Webhook.MidtransWebhook |
| POST   | `/api/v1/webhooks/xendit`   | Webhook.XenditWebhook |

### 4.2 Customer Portal (`PortalAuth` JWT)

| Method | Path | Handler |
|---|---|---|
| POST | `/portal/v1/login` | CustomerPortal.Login |
| GET  | `/portal/v1/profile` | CustomerPortal.GetProfile |
| PUT  | `/portal/v1/profile/password` | CustomerPortal.ChangePortalPassword |
| GET  | `/portal/v1/subscriptions` | CustomerPortal.GetSubscriptions |
| GET  | `/portal/v1/invoices` / `/invoices/:id` | CustomerPortal.GetInvoice(s) |
| POST | `/portal/v1/payments` | CustomerPortal.CreatePayment |
| GET  | `/portal/v1/payments` / `/payments/:id` | CustomerPortal.GetPayment(s) |
| POST | `/portal/v1/payments/:id/pay` | CustomerPortal.PayWithGateway |

### 4.3 Agent Portal (`AgentPortalAuth` JWT)

| Method | Path | Handler |
|---|---|---|
| POST | `/agent-portal/v1/login` | AgentPortal.Login |
| GET  | `/agent-portal/v1/profile` | AgentPortal.GetProfile |
| PUT  | `/agent-portal/v1/profile/password` | AgentPortal.ChangePassword |
| GET  | `/agent-portal/v1/invoices` / `:id` | AgentPortal.GetInvoice(s) |
| POST | `/agent-portal/v1/invoices/:id/request-payment` | AgentPortal.RequestPayment |
| GET  | `/agent-portal/v1/sales` | AgentPortal.GetSales |

### 4.4 Admin API `/api/v1/*` (JWT + Casbin RBAC)

**Auth**
`POST /auth/change-password` · `POST /auth/logout` · `GET /auth/me`

**Users** (admin only)
`GET /users` · `POST /users` · `GET /users/:id` · `DELETE /users/:id`

**Customers**
`GET|POST /customers` · `GET|PUT|DELETE /customers/:id`
`POST /customers/:id/activate-account` · `POST /customers/:id/deactivate-account`

**Routers** + nested router-scoped resources
`GET|POST /routers` · `GET|PUT|DELETE /routers/:router_id`
`GET /routers/selected` · `POST /routers/select/:id`
`POST /routers/sync-all`
`POST /routers/:router_id/sync` · `POST /routers/:router_id/test-connection`

**BandwidthProfiles** *(router-scoped)*
`GET|POST /routers/:router_id/bandwidth-profiles`
`GET|PUT|DELETE /routers/:router_id/bandwidth-profiles/:id`

**Subscriptions** *(router-scoped)*
`GET|POST /routers/:router_id/subscriptions`
`GET|PUT|DELETE /routers/:router_id/subscriptions/:id`
`POST .../activate · .../isolate · .../restore · .../suspend · .../terminate`

**HotspotSales**
`GET /routers/:router_id/hotspot-sales`

**Mikhmon** *(router-scoped)* — Expire, Profile, Report, Voucher

**MikroTik API** *(router-scoped)* — PPP · Hotspot · Network · Monitor · Collector · Raw

**SalesAgents**
`GET|POST /sales-agents` · `GET|PUT|DELETE /sales-agents/:id`
`GET /sales-agents/:id/profile-prices`
`PUT /sales-agents/:id/profile-prices/:profile`
`GET|POST /sales-agents/:id/invoices` (+ `/generate`)

**AgentInvoices (global)**
`GET /agent-invoices` · `GET /agent-invoices/:id`
`PUT .../pay · .../cancel` · `POST /agent-invoices/process`

**Invoices**
`GET /invoices` · `GET /invoices/overdue` · `GET /invoices/:id`
`DELETE /invoices/:id` · `POST /invoices/trigger-monthly`

**Payments**
`GET|POST /payments` · `GET /payments/:id`
`POST /payments/:id/confirm · .../reject · .../refund · .../initiate-gateway`

**Registrations**
`GET /registrations` · `GET /registrations/:id`
`POST /registrations/:id/approve · .../reject`

**Cash management**
`GET|POST /cash-entries` · `GET|PUT|DELETE /cash-entries/:id`
`POST /cash-entries/:id/approve · .../reject`
`GET|POST /petty-cash` · `GET|PUT /petty-cash/:id` · `POST /petty-cash/:id/topup`

**Reports**
`GET /reports/summary` · `GET /reports/subscriptions`
`GET /reports/cash-flow` · `GET /reports/cash-balance` · `GET /reports/reconciliation`

**SystemSettings**
`GET|PUT /settings` · `GET /settings/:id`

---

## 5. State Machines (for State Diagrams)

### 5.1 Subscription
```
[*] → pending
pending    → active        (activate)        — writes PPP secret to RouterOS
active     → suspended     (suspend)
active     → isolated      (isolate, auto-isolate on overdue)
active     → terminated    (terminate)
active     → expired       (scheduler on expiry_date)
suspended  → active        (restore)
isolated   → active        (restore, e.g. on payment)
expired    → active        (renew + payment)
any        → terminated    (terminate)
terminated → [*]
```

### 5.2 Invoice
```
[*] → draft → sent → unpaid
unpaid  → partial  (partial payment)
unpaid  → paid     (full payment)
partial → paid     (remaining paid)
partial → overpaid
paid    → overpaid (excess payment)
unpaid  → overdue  (past due_date)
overdue → paid
any     → cancelled
paid    → refunded
```

### 5.3 Payment
```
[*] → pending
pending   → confirmed (manual admin or webhook)
pending   → rejected  (admin reject)
confirmed → refunded  (full/partial refund)
```

### 5.4 CustomerRegistration
```
[*] → pending
pending → approved   (creates Customer + optional Subscription)
pending → rejected   (rejection_reason required)
```

### 5.5 AgentInvoice
```
[*] → draft → unpaid
unpaid → paid
unpaid → cancelled
```

### 5.6 CashEntry
```
[*] → pending
pending → approved  (updates petty_cash_fund.current_balance if linked)
pending → rejected
```

---

## 6. Sequence Diagram Source Flows

### 6.1 Login (admin)
```
Client → POST /api/v1/auth/login (email, password)
  AuthHandler → AuthService.Login
    UserRepo.FindByEmail → *User
    bcrypt.CompareHashAndPassword
    JWT.Service.Generate(userID, role, bearerKey)
    UserRepo.UpdateLastLogin
    Redis.SetToken (optional session store)
  ← {access_token, refresh_token, user}
```

### 6.2 Customer Registration → Approval
```
Public POST /register
  RegistrationHandler → RegistrationService.Create
    CustomerRegistrationRepo.Create (status=pending)
    NotificationService.SendRegistrationConfirmation (WA)

Admin POST /registrations/:id/approve
  RegistrationService.Approve (TX)
    CustomerRegistrationRepo.FindByID
    CustomerService.CreateFromRegistration
      SequenceCounterRepo.Next("customer") → CUST-00001
      CustomerRepo.Create
    [optional] SubscriptionService.Create (if bandwidth_profile_id present)
      RouterService.AddPPPSecret → RouterOS API
      SubscriptionRepo.Create (status=pending→active)
    CustomerRegistrationRepo.Update (status=approved, customer_id)
    NotificationService.SendWelcome
```

### 6.3 Monthly Billing Cycle
```
Scheduler/Admin POST /invoices/trigger-monthly
  BillingService.TriggerMonthlyBilling
    SubscriptionRepo.ListActiveForBilling(month, year)
    FOR EACH subscription:
      BillingDomain.Calculate(plan, prev_invoice, settings)
        → subtotal, tax, discount, late_fee, total
      SequenceCounterRepo.Next("invoice") → INV-202604-00001
      InvoiceRepo.Create + InvoiceItemRepo.CreateBatch (TX)
      NotificationService.SendInvoiceCreated (WA template)
```

### 6.4 Payment & Allocation
```
Admin POST /payments (or Portal POST /payments)
  PaymentService.Create (TX)
    CustomerRepo.FindByID
    SequenceCounterRepo.Next("payment") → PAY-202604-...
    PaymentRepo.Create (status=pending)

Admin POST /payments/:id/confirm
  PaymentService.Confirm (TX via Transactor)
    PaymentRepo.FindByID → p
    PaymentDomain.ValidateConfirm(p)
    PaymentAllocationRepo.AllocateFIFO(customer_id, amount)
      → distributes over unpaid/partial Invoices oldest-first
      → updates Invoice.paid_amount, status
    PaymentRepo.Update (status=confirmed, allocated_amount)
    CashManagementService.RecordIncome(source=invoice, ref=payment_id)
    NotificationService.SendPaymentConfirmed
```

### 6.5 Gateway Payment (Midtrans/Xendit)
```
Portal POST /portal/v1/payments/:id/pay
  CustomerPortalHandler → PaymentService.InitiateGateway
    gatewayAdapter.CreateCharge → {payment_url, trx_id}
    PaymentRepo.Update(gateway_payment_url, gateway_trx_id)
  ← redirect URL

Gateway → POST /api/v1/webhooks/midtrans|xendit
  WebhookHandler → PaymentService.HandleWebhook
    verify signature
    PaymentRepo.FindByGatewayTrxID
    PaymentService.Confirm(id)   (same flow as 6.4)
```

### 6.6 Subscription Activation (RouterOS integration)
```
POST /routers/:router_id/subscriptions/:id/activate
  SubscriptionService.Activate
    SubscriptionRepo.FindByID
    RouterService.GetClient(router) → mikrotik client (cached in Redis)
    client.PPP.AddSecret(username, password, profile, remote-addr)
    → writes subscription.mt_ppp_id
    SubscriptionDomain.TransitionTo(active)
    SubscriptionRepo.Update(status=active, activated_at=now)
    NotificationService.SendServiceActivated
```

### 6.7 Auto-Isolate Overdue
```
Scheduler (cron)
  BillingService.ProcessOverdue
    InvoiceRepo.FindOverdue(gracePeriod)
    FOR EACH overdue invoice:
      SubscriptionService.Isolate(sub_id)
        RouterService.ChangePPPProfile(username, isolate_profile_name)
        SubscriptionRepo.Update(status=isolated)
      NotificationService.SendIsolationWarning
```

### 6.8 Hotspot Agent Sale + Invoice Generation
```
Agent via Agent Portal (or POS integration):
  POST /agent-portal/.../sales → HotspotSaleService
    RouterOS.Hotspot.AddUser(voucher)
    HotspotSaleRepo.Create(router_id, agent_id, profile, prices)

Scheduled (weekly/monthly):
  POST /agent-invoices/process
  AgentInvoiceService.ProcessScheduled
    FOR EACH active SalesAgent where period elapsed:
      HotspotSaleRepo.SumForPeriod(agent_id, start, end)
      AgentInvoiceService.Generate(agent_id, period)
        SequenceCounterRepo.Next("agent-invoice")
        AgentInvoiceRepo.Create(subtotal, selling_total, discount, total)
      NotificationService.SendAgentInvoice

PUT /agent-invoices/:id/pay
  AgentInvoiceService.MarkPaid
    AgentInvoiceRepo.Update(status=paid, paid_amount)
    CashManagementService.RecordIncome(source=agent_invoice)
```

---

## 7. Use Case Diagram Actors & Goals

### 7.1 Actors
- **SuperAdmin** — all-access, RBAC inherits from Admin
- **Admin** — full `/api/v1/*` access
- **Technician** — inherits `staff` policies (read customers, invoices, payments; field work)
- **SalesAgent** — inherits `staff` + agent portal access to own invoices/sales
- **Customer** — portal-only (own profile, subscriptions, invoices, payments)
- **Guest** — register, login, health, webhooks (non-user)
- **Scheduler/Cron** — internal actor for monthly billing, overdue sweep, agent invoices
- **PaymentGateway** — external (Midtrans, Xendit) — inbound webhooks
- **MikroTik RouterOS** — external system (PPP secrets, Hotspot users, queues)
- **WhatsApp GOWA** — external notifications
- **InfluxDB v3** — metrics/collector sink

### 7.2 Primary Use Cases
- Manage Users (Admin)
- Manage Customers (Admin)
- Manage MikroTik Routers & test connection (Admin/Tech)
- Manage Bandwidth Profiles (Admin)
- Create/Activate/Suspend/Terminate Subscriptions (Admin/Tech)
- Generate & Send Invoices (Admin / Scheduler)
- Record & Confirm Payments (Admin; Customer via portal)
- Approve/Reject Registrations (Admin)
- Manage Sales Agents & Voucher Pricing (Admin)
- Generate & Pay Agent Invoices (Admin / Scheduler)
- Cash book & Petty Cash (Admin)
- View Reports (Admin)
- Self-service: view invoices, pay online, change password (Customer)
- Agent self-service: view own invoices & sales (SalesAgent)

---

## 8. Deployment / Component Diagram Source

### 8.1 Runtime Components
- **Go binary (`mikmongo`)** — single HTTP server (Gin) exposing ports for admin/portal/agent-portal + webhooks
- **PostgreSQL** — primary store
- **Redis** — cache (router client cache, bandwidth profiles cache, token blacklist)
- **GOWA** (WhatsApp gateway) — HTTP client
- **MikroTik RouterOS devices** — managed via API (8728) / REST (80/443)
- **InfluxDB v3** — collector metrics sink (CPU/memory/interfaces)
- **Payment Gateway** — Midtrans / Xendit (webhooks in)

### 8.2 Key Packages (`pkg/`)
- `pkg/jwt` — token issue/verify
- `pkg/redis` — redis client wrapper
- `pkg/mikrotik` — RouterOS API client (PPP, Hotspot, Queue, IP, Raw)
- `pkg/crypto` — AES encrypt/decrypt for router passwords

### 8.3 Configuration Surfaces
- `.env` — DB DSN, Redis URL, JWT secret, ENC key, GOWA URL, InfluxDB URL
- `system_settings` table — runtime-tunable (invoice prefix, late fee, tax rate, WA template toggles)
- `message_templates` table — per-event WhatsApp/Email template

---

## 9. Cross-Cutting Concerns

| Concern | Implementation |
|---|---|
| **Authentication** | JWT (access + refresh) for admin, customer portal, agent portal |
| **Authorization** | Casbin RBAC (roles: superadmin, admin, technician, sales_agent, customer) |
| **Password storage** | bcrypt (users.password_hash, customers.portal_password_hash, sales_agents.password_hash) |
| **Router secrets** | AES-encrypted in `mikrotik_routers.password_encrypted` |
| **Document numbering** | `sequence_counters` with `SELECT FOR UPDATE` and optional monthly/yearly reset |
| **Audit trail** | `audit_logs` captures old/new JSON diff per entity change |
| **Soft delete** | GORM `deleted_at` on all mutable entities |
| **Transactions** | `repository.Transactor` wraps multi-model tx (billing, payments, registration approval) |
| **Notifications** | `NotificationService` resolves `message_templates` by event+channel and sends via GOWA |
| **Caching** | Redis for router clients, bandwidth profiles, auth tokens |
| **Rate limiting** | `middleware/ratelimit.go` (configurable per route) |

---

## 10. File-to-Diagram Quick Map

| Diagram Type | Primary Source Files |
|---|---|
| **Class (domain)** | `internal/model/*.go` |
| **Class (services)** | `internal/service/*.go` + `service/registry.go` |
| **Class (handlers)** | `internal/handler/*.go` + `handler/registry.go` |
| **ER diagram** | `internal/model/*.go` + `internal/migration/*.go` |
| **Component** | `cmd/mikrotik/main.go`, registries, `pkg/*` |
| **Deployment** | `deployments/`, `.env.example`, `Makefile` |
| **Sequence (auth)** | `handler/auth_handler.go`, `service/auth_service.go`, `pkg/jwt` |
| **Sequence (billing)** | `service/billing_service.go`, `domain/billing/`, `handler/billing_handler.go` |
| **Sequence (payment)** | `service/payment_service.go`, `domain/payment/`, `handler/webhook_handler.go` |
| **Sequence (subscription)** | `service/subscription_service.go`, `service/router_service.go`, `pkg/mikrotik` |
| **Use Case** | `internal/router/*.go` + `internal/casbin/enforcer.go` |
| **State (subscription/invoice/payment)** | `internal/domain/{subscription,billing,payment}/domain.go` |
| **Activity (registration)** | `domain/registration/domain.go`, `service/registration_service.go` |

---

## 11. Diagram Generation Hints (PlantUML-ready)

### 11.1 Class diagram stereotypes
- `<<entity>>` — model structs
- `<<service>>` — service layer
- `<<repository>>` — repos
- `<<handler>>` — HTTP handlers
- `<<domain>>` — domain logic (pure)
- `<<dto>>` — request/response types
- `<<middleware>>` — middlewares
- `<<external>>` — RouterOS, Midtrans, Xendit, GOWA, InfluxDB

### 11.2 Suggested packages (PlantUML)
```
package "handler" { ... }
package "service" { ... }
package "domain" { ... }
package "repository" { ... }
package "model" { ... }
package "middleware" { ... }
cloud "RouterOS" { ... }
cloud "PaymentGateway" { ... }
cloud "GOWA" { ... }
database "PostgreSQL" { ... }
database "Redis" { ... }
database "InfluxDB" { ... }
```

### 11.3 Recommended relationship edges
- `Handler --> Service` (uses)
- `Service --> Domain` (uses, stateless)
- `Service --> Repository` (uses interface)
- `Repository ..> Model` (persists)
- `Service --> Service` (cross-service via setter injection — dashed)
- `Middleware --> Service` (for Auth/RBAC)
- `Service --> External` (via pkg/*)

---

**End of UML Knowledge Base.** Use this as a single-source reference when generating any UML diagram for the MikMongo project.
