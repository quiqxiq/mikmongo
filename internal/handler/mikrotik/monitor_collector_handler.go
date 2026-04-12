package mikrotik

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mikmongo/internal/service"
)

// CollectorStatusHandler exposes collector status via HTTP.
type CollectorStatusHandler struct {
	CollectorSvc *service.CollectorService
}

// GetStatus returns the status of all active collectors.
func (h *CollectorStatusHandler) GetStatus(c *gin.Context) {
	status := h.CollectorSvc.Status()
	c.JSON(http.StatusOK, gin.H{"data": status})
}
