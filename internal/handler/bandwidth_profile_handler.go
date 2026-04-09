package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mikmongo/internal/dto"
	"mikmongo/internal/service"
	"mikmongo/pkg/response"
)

// BandwidthProfileHandler handles bandwidth profile HTTP requests
type BandwidthProfileHandler struct {
	service *service.BandwidthProfileService
}

// NewBandwidthProfileHandler creates a new bandwidth profile handler
func NewBandwidthProfileHandler(svc *service.BandwidthProfileService) *BandwidthProfileHandler {
	return &BandwidthProfileHandler{service: svc}
}

// Create handles bandwidth profile creation for a specific router
func (h *BandwidthProfileHandler) Create(c *gin.Context) {
	routerID, err := uuid.Parse(c.Param("router_id"))
	if err != nil {
		response.BadRequest(c, "invalid router_id")
		return
	}

	var req dto.CreateBandwidthProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check for duplicate name on this router before attempting MikroTik sync
	existing, _ := h.service.GetByRouterAndName(c.Request.Context(), routerID, req.Name)
	if existing != nil {
		response.Conflict(c, fmt.Sprintf("bandwidth profile with name %q already exists on this router", req.Name))
		return
	}

	profile := req.ToModel(routerID.String())

	mtCfg := &service.PPPProfileConfig{
		LocalAddress:   req.MtLocalAddress,
		RemoteAddress:  req.MtRemoteAddress,
		ParentQueue:    req.MtParentQueue,
		QueueType:      req.MtQueueType,
		DNSServer:      req.MtDNSServer,
		SessionTimeout: req.MtSessionTimeout,
		IdleTimeout:    req.MtIdleTimeout,
	}

	if err := h.service.Create(c.Request.Context(), profile, mtCfg); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Created(c, dto.ProfileToResponse(profile, nil))
}

// Get handles getting a bandwidth profile by ID, enriched with live MikroTik data.
func (h *BandwidthProfileHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	profile, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	pppData, _ := h.service.GetPPPProfile(c.Request.Context(), profile) // nil OK — graceful degradation
	response.OK(c, dto.ProfileToResponse(profile, pppData))
}

// List handles listing bandwidth profiles for a specific router
func (h *BandwidthProfileHandler) List(c *gin.Context) {
	routerID, err := uuid.Parse(c.Param("router_id"))
	if err != nil {
		response.BadRequest(c, "invalid router_id")
		return
	}

	limit, offset := getPagination(c)
	profiles, count, err := h.service.ListByRouterID(c.Request.Context(), routerID, limit, offset)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	mtMap := h.service.GetPPPProfiles(c.Request.Context(), profiles)
	responses := make([]dto.BandwidthProfileResponse, len(profiles))
	for i := range profiles {
		responses[i] = dto.ProfileToResponse(&profiles[i], mtMap[profiles[i].Name])
	}
	response.WithMeta(c, http.StatusOK, responses, &response.Meta{Total: count, Limit: limit, Offset: offset})
}

// Update handles updating a bandwidth profile
func (h *BandwidthProfileHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	profile, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	var req dto.UpdateBandwidthProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	req.ApplyTo(profile)

	mtCfg := &service.PPPProfileConfig{
		LocalAddress:   req.MtLocalAddress,
		RemoteAddress:  req.MtRemoteAddress,
		ParentQueue:    req.MtParentQueue,
		QueueType:      req.MtQueueType,
		DNSServer:      req.MtDNSServer,
		SessionTimeout: req.MtSessionTimeout,
		IdleTimeout:    req.MtIdleTimeout,
	}

	if err := h.service.Update(c.Request.Context(), profile, mtCfg); err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.OK(c, dto.ProfileToResponse(profile, nil))
}

// Delete handles deleting a bandwidth profile
func (h *BandwidthProfileHandler) Delete(c *gin.Context) {
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
