package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	casbinpkg "mikmongo/internal/casbin"
	"mikmongo/internal/config"
	"mikmongo/internal/domain"
	"mikmongo/internal/handler"
	mikrotikHandler "mikmongo/internal/handler/mikrotik"
	"mikmongo/internal/handler/mikrotik/mikhmon"
	"mikmongo/internal/middleware"
	_ "mikmongo/internal/migration"
	"mikmongo/internal/queue"
	"mikmongo/internal/queue/consumer"
	"mikmongo/internal/repository"
	"mikmongo/internal/repository/postgres"
	"mikmongo/internal/router"
	"mikmongo/internal/scheduler"
	"mikmongo/internal/seeder"
	"mikmongo/internal/notification"
	"mikmongo/internal/service"
	"mikmongo/internal/service/mikrotik"
	"mikmongo/pkg/gowa"
	"mikmongo/pkg/jwt"
	"mikmongo/pkg/logger"
	xenditpkg "mikmongo/pkg/payment/xendit"
	"mikmongo/pkg/rabbitmq"
	"mikmongo/pkg/redis"
	"mikmongo/pkg/ws"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"go.uber.org/zap"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	logg := logger.New(cfg.App.Env)
	defer logg.Sync()

	logg.Info("Starting MikMongo server...",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	if err := validateCriticalConfig(cfg); err != nil {
		logg.Fatal("Invalid configuration", zap.Error(err))
	}

	// Configure WebSocket allowed origins from environment
	ws.SetAllowedOrigins(cfg.App.AllowedOrigins)

	// Database
	db, err := gorm.Open(gormpg.Open(cfg.GetDSN()), &gorm.Config{})
	if err != nil {
		logg.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Auto-migrate + seed (when AUTO_MIGRATE=true)
	// Drops all tables, re-runs all migrations from scratch, then seeds.
	if cfg.Seed.AutoMigrate {
		sqlDB, err := db.DB()
		if err != nil {
			logg.Fatal("Failed to get sql.DB for migration", zap.Error(err))
		}
		if err := goose.SetDialect("postgres"); err != nil {
			logg.Fatal("Failed to set goose dialect", zap.Error(err))
		}

		// Reset: roll back ALL migrations (drops tables in reverse order)
		logg.Info("Resetting database (dropping all tables)...")
		if err := goose.Reset(sqlDB, "."); err != nil {
			logg.Warn("goose.Reset failed (first run?), continuing...", zap.Error(err))
		}

		// Up: re-run all migrations from scratch
		logg.Info("Running all migrations from scratch...")
		if err := goose.Up(sqlDB, "."); err != nil {
			logg.Fatal("Failed to run migrations", zap.Error(err))
		}
		logg.Info("Migrations completed")

		// Seed all data
		seedCfg := seeder.Config{
			EncryptionKey: cfg.JWT.Secret,
		}
		if err := seeder.New(sqlDB, seedCfg).Run(context.Background()); err != nil {
			logg.Fatal("Failed to run seeder", zap.Error(err))
		}
		logg.Info("Seed completed")
	}

	// Redis
	redisClient := redis.NewClient(redis.Options{
		Host:     cfg.Redis.Host,
		Port:     parseInt(cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// RabbitMQ
	var rabbitClient *rabbitmq.Client
	rabbitClient, err = rabbitmq.NewClient(
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
		cfg.RabbitMQ.User,
		cfg.RabbitMQ.Password,
	)
	if err != nil {
		logg.Error("Failed to connect to RabbitMQ", zap.Error(err))
	} else {
		defer rabbitClient.Close()
	}

	// JWT
	jwtExpiry, _ := time.ParseDuration(cfg.JWT.Expiry)
	jwtRefreshExpiry, _ := time.ParseDuration(cfg.JWT.RefreshExpiry)
	if jwtRefreshExpiry == 0 {
		jwtRefreshExpiry = 7 * 24 * time.Hour
	}
	jwtService := jwt.NewService(cfg.JWT.Secret, jwtExpiry, jwtRefreshExpiry)

	// Repositories
	pgRepo := postgres.NewRepository(db)
	repoRegistry := &repository.Registry{
		CustomerRepo:             pgRepo.CustomerRepo,
		InvoiceRepo:              pgRepo.InvoiceRepo,
		PaymentRepo:              pgRepo.PaymentRepo,
		RouterDeviceRepo:         pgRepo.RouterDeviceRepo,
		UserRepo:                 pgRepo.UserRepo,
		BandwidthProfileRepo:     pgRepo.BandwidthProfileRepo,
		SubscriptionRepo:         pgRepo.SubscriptionRepo,
		CustomerRegistrationRepo: pgRepo.CustomerRegistrationRepo,
		InvoiceItemRepo:          pgRepo.InvoiceItemRepo,
		PaymentAllocationRepo:    pgRepo.PaymentAllocationRepo,
		SystemSettingRepo:        pgRepo.SystemSettingRepo,
		SequenceCounterRepo:      pgRepo.SequenceCounterRepo,
		MessageTemplateRepo:      pgRepo.MessageTemplateRepo,
		AuditLogRepo:             pgRepo.AuditLogRepo,
		Transactor:               pgRepo.Transactor,
		HotspotSaleRepo:          pgRepo.HotspotSaleRepo,
		SalesAgentRepo:           pgRepo.SalesAgentRepo,
		AgentInvoiceRepo:         pgRepo.AgentInvoiceRepo,
		CashEntryRepo:            pgRepo.CashEntryRepo,
		PettyCashFundRepo:        pgRepo.PettyCashFundRepo,
	}

	// Domains
	domainRegistry := domain.NewRegistry(
		domain.NewCustomerDomain(),
		domain.NewBillingDomain(),
		domain.NewPaymentDomain(),
		domain.NewRouterDomain(),
		domain.NewSubscriptionDomain(),
		domain.NewRegistrationDomain(),
		domain.NewNotificationDomain(),
	)

	// Queue
	queueRegistry := queue.NewRegistry(rabbitClient)

	// GoWA WhatsApp client
	var waClient notification.WhatsAppSender
	if cfg.GoWA.Username != "" {
		gowaClient := gowa.New(&gowa.Config{
			BaseURL:  cfg.GoWA.BaseURL,
			Username: cfg.GoWA.Username,
			Password: cfg.GoWA.Password,
			DeviceID: cfg.GoWA.DeviceID,
			Timeout:  time.Duration(cfg.GoWA.Timeout) * time.Second,
		})
		waClient = notification.NewGoWAClient(gowaClient, cfg.GoWA.GroupID)
	}

	// Services
	encKey := cfg.JWT.Secret
	serviceRegistry := service.NewRegistry(
		repoRegistry,
		domainRegistry,
		jwtService,
		encKey,
		db,
		redisClient,
		logg.Logger,
		waClient,
	)

	// Initialize Mikrotik service registry
	mikrotikRegistry := mikrotik.NewRegistry(serviceRegistry.Router)
	serviceRegistry.Mikrotik = mikrotikRegistry

	// Inject services into queue consumers (callbacks, avoids import cycle)
	queueRegistry.BillingConsumer.SetHandler(func(ctx context.Context, subscriptionID uuid.UUID, period time.Time) error {
		_, err := serviceRegistry.Billing.GenerateInvoice(ctx, subscriptionID, period)
		return err
	})
	queueRegistry.SuspendConsumer.SetHandler(func(ctx context.Context, customerID uuid.UUID) error {
		return serviceRegistry.Customer.IsolateAllSubscriptions(ctx, customerID)
	})
	queueRegistry.NotificationConsumer.SetHandler(&consumer.NotificationHandler{
		SendWhatsApp: func(ctx context.Context, phone, message string) error {
			return serviceRegistry.Notification.SendViaWhatsApp(ctx, phone, message)
		},
		SendEmail: func(ctx context.Context, to, subject, body string) error {
			return serviceRegistry.Notification.SendViaEmail(ctx, to, subject, body)
		},
	})

	// Handlers
	// Payment gateway providers
	xenditClient := xenditpkg.New(xenditpkg.Config{
		SecretKey:    cfg.Xendit.SecretKey,
		WebhookToken: cfg.Xendit.WebhookToken,
	})

	handlerRegistry := handler.NewRegistry(serviceRegistry, repoRegistry.SystemSettingRepo, jwtService)
	handlerRegistry.Payment.SetProvider("xendit", xenditClient)
	handlerRegistry.CustomerPortal.SetProvider("xendit", xenditClient)
	handlerRegistry.Webhook.SetXenditProvider(xenditClient)

	// Collector Service
	collectorSvc := service.NewCollectorService(
		serviceRegistry.Router,
		cfg.Redis.Host+":"+cfg.Redis.Port,
		cfg.InfluxDB.URL,
		cfg.InfluxDB.Token,
		cfg.InfluxDB.Org,
		cfg.InfluxDB.Bucket,
		logg.Logger,
	)

	// Initialize MikroTik handler registry
	handlerRegistry.Mikrotik = mikrotikHandler.NewRegistry(mikrotikRegistry, serviceRegistry.Router, redisClient.GoRedis())
	handlerRegistry.Mikrotik.CollectorStatus = &mikrotikHandler.CollectorStatusHandler{CollectorSvc: collectorSvc}
	handlerRegistry.Mikhmon = mikhmon.NewRegistry(mikrotikRegistry)

	// Initialize HotspotSale + SalesAgent handlers
	hotspotSaleSvc := service.NewHotspotSaleService(
		mikrotikRegistry.Mikhmon.Voucher,
		pgRepo.HotspotSaleRepo,
		pgRepo.SalesAgentRepo,
	)
	handlerRegistry.HotspotSale = handler.NewHotspotSaleHandler(hotspotSaleSvc)
	handlerRegistry.SalesAgent = handler.NewSalesAgentHandler(pgRepo.SalesAgentRepo, pgRepo.SystemSettingRepo)
	handlerRegistry.AgentInvoice = handler.NewAgentInvoiceHandler(serviceRegistry.AgentInvoice)
	handlerRegistry.AgentPortal = handler.NewAgentPortalHandler(pgRepo.SalesAgentRepo, serviceRegistry.AgentInvoice, hotspotSaleSvc, jwtService)
	handlerRegistry.CashManagement = handler.NewCashManagementHandler(serviceRegistry.CashManagement)

	// Casbin RBAC enforcer
	casbinEnforcer, err := casbinpkg.NewEnforcer(db)
	if err != nil {
		logg.Fatal("Failed to create Casbin enforcer", zap.Error(err))
	}

	// Middleware
	middlewareRegistry := middleware.NewRegistry(logg.Logger, jwtService, redisClient, casbinEnforcer, cfg.App.AllowedOrigins, cfg.InternalKey)

	// Router
	r := router.New(handlerRegistry, middlewareRegistry)

	// Schedulers
	schedulerRegistry := scheduler.NewRegistry(serviceRegistry, queueRegistry)
	schedulerRegistry.Start()
	defer schedulerRegistry.Stop()

	// HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.App.Port,
		Handler: r,
	}

	// Start collector in background
	go func() {
		if err := collectorSvc.Start(context.Background()); err != nil {
			logg.Error("Collector failed to start", zap.Error(err))
		}
	}()
	defer collectorSvc.Stop()

	go func() {
		logg.Info("Server starting", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logg.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logg.Error("Server forced to shutdown", zap.Error(err))
	}

	logg.Info("Server exited")
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func validateCriticalConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Keep local development friction low, but enforce sane secrets elsewhere.
	if cfg.App.Env != "development" {
		if cfg.JWT.Secret == "" || cfg.JWT.Secret == "your-secret-key-here" {
			return fmt.Errorf("JWT_SECRET must be set to a non-default value when APP_ENV is %q", cfg.App.Env)
		}
	}

	return nil
}
