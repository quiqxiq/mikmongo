package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mikmongo/internal/dto"
	"mikmongo/internal/service"
	"mikmongo/pkg/response"
)

// SubscriptionHandler handles subscription HTTP requests
type SubscriptionHandler struct {
	service *service.SubscriptionService
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(svc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: svc}
}

// Create handles subscription creation for a specific router
func (h *SubscriptionHandler) Create(c *gin.Context) {
	routerID, err := uuid.Parse(c.Param("router_id"))
	if err != nil {
		response.BadRequest(c, "invalid router_id")
		return
	}

	var req dto.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sub := req.ToModel(routerID.String())

	mtCfg := &service.PPPSecretConfig{
		Service:        req.MtService,
		LocalAddress:   req.MtLocalAddress,
		RemoteAddress:  req.MtRemoteAddress,
		Comment:        req.MtComment,
		Routes:         req.MtRoutes,
		LimitBytesIn:   req.MtLimitBytesIn,
		LimitBytesOut:  req.MtLimitBytesOut,
	}

	if err := h.service.Create(c.Request.Context(), sub, mtCfg); err != nil {
		if errors.Is(err, service.ErrMikrotikPPPSecretExists) {
			response.Conflict(c, err.Error())
			return
		}
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, dto.SubscriptionToResponse(sub, nil))
}

// Get handles getting a subscription by ID, enriched with live MikroTik data.
func (h *SubscriptionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	secretData, _ := h.service.GetPPPSecret(c.Request.Context(), sub) // nil OK — graceful degradation
	response.OK(c, dto.SubscriptionToResponse(sub, secretData))
}

// List handles listing subscriptions for a specific router
func (h *SubscriptionHandler) List(c *gin.Context) {
	routerID, err := uuid.Parse(c.Param("router_id"))
	if err != nil {
		response.BadRequest(c, "invalid router_id")
		return
	}

	limit, offset := getPagination(c)
	subs, count, err := h.service.ListByRouterID(c.Request.Context(), routerID, limit, offset)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	secretMap := h.service.GetPPPSecrets(c.Request.Context(), subs)
	responses := make([]dto.SubscriptionResponse, len(subs))
	for i := range subs {
		responses[i] = dto.SubscriptionToResponse(&subs[i], secretMap[subs[i].Username])
	}
	response.WithMeta(c, http.StatusOK, responses, &response.Meta{Total: count, Limit: limit, Offset: offset})
}

// Update handles updating a subscription
func (h *SubscriptionHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	var req dto.UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	req.ApplyTo(sub)

	mtCfg := &service.PPPSecretConfig{
		Service:        req.MtService,
		LocalAddress:   req.MtLocalAddress,
		RemoteAddress:  req.MtRemoteAddress,
		Comment:        req.MtComment,
		Routes:         req.MtRoutes,
		LimitBytesIn:   req.MtLimitBytesIn,
		LimitBytesOut:  req.MtLimitBytesOut,
	}

	if err := h.service.Update(c.Request.Context(), sub, mtCfg); err != nil {
		if errors.Is(err, service.ErrMikrotikPPPSecretNotFound) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.InternalServerError(c, err.Error())
		return
	}
	response.OK(c, dto.SubscriptionToResponse(sub, nil))
}

// Delete handles deleting a subscription
func (h *SubscriptionHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// Activate handles activating a subscription
func (h *SubscriptionHandler) Activate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Activate(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "activated"})
}

// Isolate handles isolating a subscription
func (h *SubscriptionHandler) Isolate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.service.Isolate(c.Request.Context(), id, req.Reason); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "isolated"})
}

// Restore handles restoring a subscription from isolation
func (h *SubscriptionHandler) Restore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Restore(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restored"})
}

// Suspend handles suspending a subscription
func (h *SubscriptionHandler) Suspend(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.service.Suspend(c.Request.Context(), id, req.Reason); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "suspended"})
}

// Terminate handles terminating a subscription
func (h *SubscriptionHandler) Terminate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Terminate(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "terminated"})
}
