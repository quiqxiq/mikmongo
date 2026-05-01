package router

import (
	"github.com/gin-gonic/gin"
	"mikmongo/internal/handler"
	"mikmongo/internal/middleware"
)

func registerAgentPortalRoutes(r *gin.Engine, handlers *handler.Registry, mw *middleware.Registry) {
	portal := r.Group("/agent-portal/v1")
	{
		portal.POST("/login", handlers.AgentPortal.Login)

		auth := portal.Group("")
		auth.Use(mw.AgentPortalAuth.AuthenticateAgentPortal())
		{
			auth.GET("/profile", handlers.AgentPortal.GetProfile)
			auth.PUT("/profile/password", handlers.AgentPortal.ChangePassword)
			auth.GET("/invoices", handlers.AgentPortal.GetInvoices)
			auth.GET("/invoices/:id", handlers.AgentPortal.GetInvoice)
			auth.POST("/invoices/:id/request-payment", handlers.AgentPortal.RequestPayment)
			auth.GET("/sales", handlers.AgentPortal.GetSales)
		}
	}
}

// registerAgentAdminRoutes registers admin-side agent management routes.
// v1 is the /api/v1 group (already has JWT + RBAC middleware).
func registerAgentAdminRoutes(v1 *gin.RouterGroup, handlers *handler.Registry) {
	// Sales agents CRUD + profile prices
	agents := v1.Group("/sales-agents")
	{
		agents.GET("", handlers.SalesAgent.List)
		agents.POST("", handlers.SalesAgent.Create)
		agents.GET("/:id", handlers.SalesAgent.Get)
		agents.PUT("/:id", handlers.SalesAgent.Update)
		agents.DELETE("/:id", handlers.SalesAgent.Delete)
		agents.GET("/:id/profile-prices", handlers.SalesAgent.ListProfilePrices)
		agents.PUT("/:id/profile-prices/:profile", handlers.SalesAgent.UpsertProfilePrice)

		// Agent invoices (scoped to agent)
		agents.GET("/:id/invoices", handlers.AgentInvoice.ListByAgent)
		agents.POST("/:id/invoices/generate", handlers.AgentInvoice.Generate)
	}

	// Agent invoices (global)
	agentInvoices := v1.Group("/agent-invoices")
	{
		agentInvoices.GET("", handlers.AgentInvoice.List)
		agentInvoices.GET("/:id", handlers.AgentInvoice.Get)
		agentInvoices.PUT("/:id/pay", handlers.AgentInvoice.MarkPaid)
		agentInvoices.PUT("/:id/cancel", handlers.AgentInvoice.Cancel)
		agentInvoices.POST("/process", handlers.AgentInvoice.ProcessScheduled)
	}
}
