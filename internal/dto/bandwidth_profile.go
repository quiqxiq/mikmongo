package dto

import (
	"time"

	mkdomain "github.com/Butterfly-Student/go-ros/domain"

	"mikmongo/internal/model"
)

// === REQUEST ===

// CreateBandwidthProfileRequest is the request body for creating a bandwidth profile.
type CreateBandwidthProfileRequest struct {
	ProfileCode        string   `json:"profile_code" binding:"required"`
	Name               string   `json:"name" binding:"required"`
	Description        *string  `json:"description"`
	PriceMonthly       float64  `json:"price_monthly" binding:"required,gt=0"`
	DownloadSpeed      int64    `json:"download_speed" binding:"required,gt=0"`
	UploadSpeed        int64    `json:"upload_speed" binding:"required,gt=0"`
	TaxRate            *float64 `json:"tax_rate"`
	BillingCycle       *string  `json:"billing_cycle"`
	BillingDay         *int     `json:"billing_day"`
	GracePeriodDays    *int     `json:"grace_period_days"`
	IsolateProfileName *string  `json:"isolate_profile_name"`
	SortOrder          *int     `json:"sort_order"`
	IsVisible          *bool    `json:"is_visible"`
	RateLimit          *string  `json:"rate_limit"` // stored on model; synced to RouterOS rate-limit
	// MikroTik PPPProfile pass-through (not stored in DB)
	MtLocalAddress   *string `json:"mt_local_address"`
	MtRemoteAddress  *string `json:"mt_remote_address"`
	MtParentQueue    *string `json:"mt_parent_queue"`
	MtQueueType      *string `json:"mt_queue_type"`
	MtDNSServer      *string `json:"mt_dns_server"`
	MtSessionTimeout *string `json:"mt_session_timeout"`
	MtIdleTimeout    *string `json:"mt_idle_timeout"`
	// MtRateLimit overrides model rate_limit for this sync only (not persisted).
	MtRateLimit *string `json:"mt_rate_limit"`
}

// ToModel converts the create request to a model.BandwidthProfile.
func (r *CreateBandwidthProfileRequest) ToModel(routerID string) *model.BandwidthProfile {
	m := &model.BandwidthProfile{
		RouterID:      routerID,
		ProfileCode:   r.ProfileCode,
		Name:          r.Name,
		Description:   r.Description,
		PriceMonthly:  r.PriceMonthly,
		DownloadSpeed: r.DownloadSpeed,
		UploadSpeed:   r.UploadSpeed,
	}
	if r.TaxRate != nil {
		m.TaxRate = *r.TaxRate
	}
	if r.BillingCycle != nil {
		m.BillingCycle = *r.BillingCycle
	}
	if r.BillingDay != nil {
		m.BillingDay = r.BillingDay
	}
	if r.GracePeriodDays != nil {
		m.GracePeriodDays = *r.GracePeriodDays
	}
	if r.IsolateProfileName != nil {
		m.IsolateProfileName = r.IsolateProfileName
	}
	if r.SortOrder != nil {
		m.SortOrder = *r.SortOrder
	}
	if r.IsVisible != nil {
		m.IsVisible = *r.IsVisible
	} else {
		m.IsVisible = true
	}
	if r.RateLimit != nil {
		m.RateLimit = r.RateLimit
	}
	return m
}

// UpdateBandwidthProfileRequest is the request body for updating a bandwidth profile.
// All fields are pointers — only non-nil fields are applied.
type UpdateBandwidthProfileRequest struct {
	Name               *string  `json:"name"`
	Description        *string  `json:"description"`
	PriceMonthly       *float64 `json:"price_monthly"`
	DownloadSpeed      *int64   `json:"download_speed"`
	UploadSpeed        *int64   `json:"upload_speed"`
	TaxRate            *float64 `json:"tax_rate"`
	BillingCycle       *string  `json:"billing_cycle"`
	BillingDay         *int     `json:"billing_day"`
	GracePeriodDays    *int     `json:"grace_period_days"`
	IsolateProfileName *string  `json:"isolate_profile_name"`
	SortOrder          *int     `json:"sort_order"`
	IsActive           *bool    `json:"is_active"`
	IsVisible          *bool    `json:"is_visible"`
	RateLimit          *string  `json:"rate_limit"`
	// MikroTik pass-through
	MtLocalAddress   *string `json:"mt_local_address"`
	MtRemoteAddress  *string `json:"mt_remote_address"`
	MtParentQueue    *string `json:"mt_parent_queue"`
	MtQueueType      *string `json:"mt_queue_type"`
	MtDNSServer      *string `json:"mt_dns_server"`
	MtSessionTimeout *string `json:"mt_session_timeout"`
	MtIdleTimeout    *string `json:"mt_idle_timeout"`
	MtRateLimit      *string `json:"mt_rate_limit"`
}

// ApplyTo applies non-nil fields to the existing model.
func (r *UpdateBandwidthProfileRequest) ApplyTo(m *model.BandwidthProfile) {
	if r.Name != nil {
		m.Name = *r.Name
	}
	if r.Description != nil {
		m.Description = r.Description
	}
	if r.PriceMonthly != nil {
		m.PriceMonthly = *r.PriceMonthly
	}
	if r.DownloadSpeed != nil {
		m.DownloadSpeed = *r.DownloadSpeed
	}
	if r.UploadSpeed != nil {
		m.UploadSpeed = *r.UploadSpeed
	}
	if r.TaxRate != nil {
		m.TaxRate = *r.TaxRate
	}
	if r.BillingCycle != nil {
		m.BillingCycle = *r.BillingCycle
	}
	if r.BillingDay != nil {
		m.BillingDay = r.BillingDay
	}
	if r.GracePeriodDays != nil {
		m.GracePeriodDays = *r.GracePeriodDays
	}
	if r.IsolateProfileName != nil {
		m.IsolateProfileName = r.IsolateProfileName
	}
	if r.SortOrder != nil {
		m.SortOrder = *r.SortOrder
	}
	if r.IsActive != nil {
		m.IsActive = *r.IsActive
	}
	if r.IsVisible != nil {
		m.IsVisible = *r.IsVisible
	}
	if r.RateLimit != nil {
		m.RateLimit = r.RateLimit
	}
}

// === RESPONSE ===

// PPPProfileInfo holds live MikroTik PPPProfile fields returned in responses.
type PPPProfileInfo struct {
	RateLimit      string `json:"rate_limit,omitempty"`
	LocalAddress   string `json:"local_address,omitempty"`
	RemoteAddress  string `json:"remote_address,omitempty"`
	ParentQueue    string `json:"parent_queue,omitempty"`
	QueueType      string `json:"queue_type,omitempty"`
	DNSServer      string `json:"dns_server,omitempty"`
	SessionTimeout string `json:"session_timeout,omitempty"`
	IdleTimeout    string `json:"idle_timeout,omitempty"`
}

// BandwidthProfileResponse is the safe response struct (no sensitive/internal fields).
type BandwidthProfileResponse struct {
	ID                 string          `json:"id"`
	RouterID           string          `json:"router_id"`
	ProfileCode        string          `json:"profile_code"`
	Name               string          `json:"name"`
	Description        *string         `json:"description,omitempty"`
	DownloadSpeed      int64           `json:"download_speed"`
	UploadSpeed        int64           `json:"upload_speed"`
	PriceMonthly       float64         `json:"price_monthly"`
	TaxRate            float64         `json:"tax_rate"`
	BillingCycle       string          `json:"billing_cycle"`
	BillingDay         *int            `json:"billing_day,omitempty"`
	IsActive           bool            `json:"is_active"`
	IsVisible          bool            `json:"is_visible"`
	SortOrder          int             `json:"sort_order"`
	GracePeriodDays    int             `json:"grace_period_days"`
	IsolateProfileName *string         `json:"isolate_profile_name,omitempty"`
	RateLimit          *string         `json:"rate_limit,omitempty"` // persisted limit string; live value also under mikrotik.rate_limit
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	Mikrotik           *PPPProfileInfo `json:"mikrotik,omitempty"` // nil when router unreachable
}

// === CONVERTERS ===

// ProfileToResponse converts a model + optional live MikroTik data to a response DTO.
// mt may be nil — Mikrotik field will be omitted from JSON.
func ProfileToResponse(m *model.BandwidthProfile, mt *mkdomain.PPPProfile) BandwidthProfileResponse {
	r := BandwidthProfileResponse{
		ID:                 m.ID,
		RouterID:           m.RouterID,
		ProfileCode:        m.ProfileCode,
		Name:               m.Name,
		Description:        m.Description,
		DownloadSpeed:      m.DownloadSpeed,
		UploadSpeed:        m.UploadSpeed,
		PriceMonthly:       m.PriceMonthly,
		TaxRate:            m.TaxRate,
		BillingCycle:       m.BillingCycle,
		BillingDay:         m.BillingDay,
		IsActive:           m.IsActive,
		IsVisible:          m.IsVisible,
		SortOrder:          m.SortOrder,
		GracePeriodDays:    m.GracePeriodDays,
		IsolateProfileName: m.IsolateProfileName,
		RateLimit:          m.RateLimit,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
	if mt != nil {
		r.Mikrotik = &PPPProfileInfo{
			RateLimit:      mt.RateLimit,
			LocalAddress:   mt.LocalAddress,
			RemoteAddress:  mt.RemoteAddress,
			ParentQueue:    mt.ParentQueue,
			QueueType:      mt.QueueType,
			DNSServer:      mt.DNSServer,
			SessionTimeout: mt.SessionTimeout,
			IdleTimeout:    mt.IdleTimeout,
		}
	}
	return r
}
