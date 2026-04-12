package mikrotik

import (
	"github.com/gin-gonic/gin"
	"mikmongo/internal/handler"
)

// RegisterCollectorRoutes registers collector cache and status routes scoped to /api/v1/routers/:router_id
func RegisterCollectorRoutes(routerGroup *gin.RouterGroup, handlers *handler.Registry) {
	if handlers.Mikrotik == nil {
		return
	}

	// Cache-first reads (per router)
	cached := routerGroup.Group("/cached")
	{
		cached.GET("/ppp-active", handlers.Mikrotik.Cached.GetCachedPPPActive)
		cached.GET("/hotspot-active", handlers.Mikrotik.Cached.GetCachedHotspotActive)
		cached.GET("/ppp-secrets", handlers.Mikrotik.Cached.GetCachedPPPSecrets)
		cached.GET("/queue-stats", handlers.Mikrotik.Cached.GetCachedQueueStats)
	}

	// Router logs from Redis Stream
	routerGroup.GET("/logs", handlers.Mikrotik.Log.GetRouterLogs)

	// Collector status (per router group but applies globally)
	if handlers.Mikrotik.CollectorStatus != nil {
		routerGroup.GET("/collector/status", handlers.Mikrotik.CollectorStatus.GetStatus)
	}
}
