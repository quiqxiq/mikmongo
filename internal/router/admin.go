package router

import (
	"github.com/gin-gonic/gin"
	"mikmongo/internal/handler"
	mikrotikRouter "mikmongo/internal/router/mikrotik"
)

func registerAdminRoutes(v1 *gin.RouterGroup, handlers *handler.Registry) {
	// Auth
	v1.POST("/auth/change-password", handlers.Auth.ChangePassword)
	v1.POST("/auth/logout", handlers.Auth.Logout)
	v1.GET("/auth/me", handlers.Auth.GetMe)

	// Users
	users := v1.Group("/users")
	{
		users.GET("", handlers.User.List)
		users.POST("", handlers.User.Create)
		users.GET("/:id", handlers.User.Get)
		users.DELETE("/:id", handlers.User.Delete)
	}

	// Customers
	customers := v1.Group("/customers")
	{
		customers.GET("", handlers.Customer.List)
		customers.POST("", handlers.Customer.Create)
		customers.GET("/:id", handlers.Customer.Get)
		customers.PUT("/:id", handlers.Customer.Update)
		customers.DELETE("/:id", handlers.Customer.Delete)
		customers.POST("/:id/activate-account", handlers.Customer.ActivateAccount)
		customers.POST("/:id/deactivate-account", handlers.Customer.DeactivateAccount)
	}

	// Routers with nested resources
	routerGroup := v1.Group("/routers")
	{
		routerGroup.GET("", handlers.Router.List)
		routerGroup.POST("", handlers.Router.Create)
		routerGroup.GET("/selected", handlers.Router.GetSelectedRouter)
		routerGroup.POST("/select/:id", handlers.Router.SelectRouter)
		routerGroup.POST("/sync-all", handlers.Router.SyncAll)

		// Router-specific routes
		router := routerGroup.Group("/:router_id")
		{
			router.GET("", handlers.Router.GetDevice)
			router.PUT("", handlers.Router.Update)
			router.DELETE("", handlers.Router.Delete)
			router.POST("/sync", handlers.Router.SyncDevice)
			router.POST("/test-connection", handlers.Router.TestConnection)

			// Bandwidth profiles (scoped to router)
			profiles := router.Group("/bandwidth-profiles")
			{
				profiles.GET("", handlers.BandwidthProfile.List)
				profiles.POST("", handlers.BandwidthProfile.Create)
				profiles.GET("/:id", handlers.BandwidthProfile.Get)
				profiles.PUT("/:id", handlers.BandwidthProfile.Update)
				profiles.DELETE("/:id", handlers.BandwidthProfile.Delete)
			}

			// Subscriptions (scoped to router)
			subs := router.Group("/subscriptions")
			{
				subs.GET("", handlers.Subscription.List)
				subs.POST("", handlers.Subscription.Create)
				subs.GET("/:id", handlers.Subscription.Get)
				subs.PUT("/:id", handlers.Subscription.Update)
				subs.DELETE("/:id", handlers.Subscription.Delete)
				subs.POST("/:id/activate", handlers.Subscription.Activate)
				subs.POST("/:id/isolate", handlers.Subscription.Isolate)
				subs.POST("/:id/restore", handlers.Subscription.Restore)
				subs.POST("/:id/suspend", handlers.Subscription.Suspend)
				subs.POST("/:id/terminate", handlers.Subscription.Terminate)
			}

			// Hotspot sales (scoped to router)
			router.GET("/hotspot-sales", handlers.HotspotSale.ListByRouter)

			// Mikhmon (scoped to router)
			mikrotikRouter.RegisterMikhmonRoutes(router, handlers)

			// MikroTik API routes (PPP, Hotspot, Network, Monitor, Raw)
			mikrotikRouter.RegisterPPPRoutes(router, handlers)
			mikrotikRouter.RegisterHotspotRoutes(router, handlers)
			mikrotikRouter.RegisterNetworkRoutes(router, handlers)
			mikrotikRouter.RegisterMonitorRoutes(router, handlers)
			mikrotikRouter.RegisterCollectorRoutes(router, handlers)
			mikrotikRouter.RegisterRawRoutes(router, handlers)
		}
	}

	// Agent management (sales agents, invoices, hotspot sales)
	registerAgentAdminRoutes(v1, handlers)

	// Invoices
	invoices := v1.Group("/invoices")
	{
		invoices.GET("", handlers.Billing.ListInvoices)
		invoices.GET("/overdue", handlers.Billing.GetOverdue)
		invoices.GET("/:id", handlers.Billing.GetInvoice)
		invoices.DELETE("/:id", handlers.Billing.CancelInvoice)
		invoices.POST("/trigger-monthly", handlers.Billing.TriggerMonthlyBilling)
	}

	// Payments
	payments := v1.Group("/payments")
	{
		payments.GET("", handlers.Payment.List)
		payments.POST("", handlers.Payment.Create)
		payments.GET("/:id", handlers.Payment.Get)
		payments.POST("/:id/confirm", handlers.Payment.Confirm)
		payments.POST("/:id/reject", handlers.Payment.Reject)
		payments.POST("/:id/refund", handlers.Payment.Refund)
		payments.POST("/:id/initiate-gateway", handlers.Payment.InitiateGateway)
	}

	// Registrations
	registrations := v1.Group("/registrations")
	{
		registrations.GET("", handlers.Registration.List)
		registrations.GET("/:id", handlers.Registration.Get)
		registrations.POST("/:id/approve", handlers.Registration.Approve)
		registrations.POST("/:id/reject", handlers.Registration.Reject)
	}

	// Cash entries
	cashEntries := v1.Group("/cash-entries")
	{
		cashEntries.GET("", handlers.CashManagement.ListEntries)
		cashEntries.POST("", handlers.CashManagement.CreateEntry)
		cashEntries.GET("/:id", handlers.CashManagement.GetEntry)
		cashEntries.PUT("/:id", handlers.CashManagement.UpdateEntry)
		cashEntries.DELETE("/:id", handlers.CashManagement.DeleteEntry)
		cashEntries.POST("/:id/approve", handlers.CashManagement.ApproveEntry)
		cashEntries.POST("/:id/reject", handlers.CashManagement.RejectEntry)
	}

	// Petty cash
	pettyCash := v1.Group("/petty-cash")
	{
		pettyCash.GET("", handlers.CashManagement.ListFunds)
		pettyCash.POST("", handlers.CashManagement.CreateFund)
		pettyCash.GET("/:id", handlers.CashManagement.GetFund)
		pettyCash.PUT("/:id", handlers.CashManagement.UpdateFund)
		pettyCash.POST("/:id/topup", handlers.CashManagement.TopUpFund)
	}

	// Reports
	reports := v1.Group("/reports")
	{
		reports.GET("/summary", handlers.Report.GetSummary)
		reports.GET("/subscriptions", handlers.Report.GetSubscriptions)
		reports.GET("/cash-flow", handlers.CashManagement.GetCashFlow)
		reports.GET("/cash-balance", handlers.CashManagement.GetCashBalance)
		reports.GET("/reconciliation", handlers.CashManagement.GetReconciliation)
	}

	// System settings
	settings := v1.Group("/settings")
	{
		settings.GET("", handlers.SystemSetting.List)
		settings.GET("/:id", handlers.SystemSetting.Get)
		settings.PUT("", handlers.SystemSetting.Upsert)
	}
}
