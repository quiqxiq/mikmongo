package dto

import (
	"time"

	mkdomain "github.com/Butterfly-Student/go-ros/domain"

	"mikmongo/internal/model"
)

// === REQUEST ===

// CreateSubscriptionRequest is the request body for creating a subscription.
type CreateSubscriptionRequest struct {
	CustomerID      string  `json:"customer_id" binding:"required,uuid"`
	PlanID          string  `json:"plan_id" binding:"required,uuid"`
	Username        string  `json:"username" binding:"required"`
	Password        *string `json:"password"`
	StaticIP        *string `json:"static_ip"`
	Gateway         *string `json:"gateway"`
	BillingDay      *int    `json:"billing_day"`
	AutoIsolate     *bool   `json:"auto_isolate"`
	GracePeriodDays *int    `json:"grace_period_days"`
	Notes           *string `json:"notes"`
	// MikroTik PPPSecret pass-through (not stored in DB)
	MtService        *string `json:"mt_service"`
	MtLocalAddress   *string `json:"mt_local_address"`
	MtRemoteAddress  *string `json:"mt_remote_address"` // RouterOS remote-address (overrides static_ip when set)
	MtComment        *string `json:"mt_comment"`       // RouterOS comment (overrides default sub:<id>)
	MtRoutes         *string `json:"mt_routes"`
	MtLimitBytesIn   *int64  `json:"mt_limit_bytes_in"`
	MtLimitBytesOut  *int64  `json:"mt_limit_bytes_out"`
}

// ToModel converts the create request to a model.Subscription.
func (r *CreateSubscriptionRequest) ToModel(routerID string) *model.Subscription {
	m := &model.Subscription{
		CustomerID: r.CustomerID,
		PlanID:     r.PlanID,
		RouterID:   routerID,
		Username:   r.Username,
		StaticIP:   r.StaticIP,
		Gateway:    r.Gateway,
		BillingDay: r.BillingDay,
		Notes:      r.Notes,
		Status:     "pending",
	}
	if r.Password != nil {
		m.Password = *r.Password
	}
	if r.AutoIsolate != nil {
		m.AutoIsolate = *r.AutoIsolate
	} else {
		m.AutoIsolate = true
	}
	if r.GracePeriodDays != nil {
		m.GracePeriodDays = r.GracePeriodDays
	}
	return m
}

// UpdateSubscriptionRequest is the request body for updating a subscription.
// All fields are pointers — only non-nil fields are applied.
type UpdateSubscriptionRequest struct {
	PlanID          *string `json:"plan_id"`
	Password        *string `json:"password"`
	StaticIP        *string `json:"static_ip"`
	Gateway         *string `json:"gateway"`
	BillingDay      *int    `json:"billing_day"`
	AutoIsolate     *bool   `json:"auto_isolate"`
	GracePeriodDays *int    `json:"grace_period_days"`
	Notes           *string `json:"notes"`
	// MikroTik pass-through
	MtService        *string `json:"mt_service"`
	MtLocalAddress   *string `json:"mt_local_address"`
	MtRemoteAddress  *string `json:"mt_remote_address"`
	MtComment        *string `json:"mt_comment"`
	MtRoutes         *string `json:"mt_routes"`
	MtLimitBytesIn   *int64  `json:"mt_limit_bytes_in"`
	MtLimitBytesOut  *int64  `json:"mt_limit_bytes_out"`
}

// ApplyTo applies non-nil fields to the existing model.
func (r *UpdateSubscriptionRequest) ApplyTo(m *model.Subscription) {
	if r.PlanID != nil {
		m.PlanID = *r.PlanID
	}
	if r.Password != nil {
		m.Password = *r.Password
	}
	if r.StaticIP != nil {
		m.StaticIP = r.StaticIP
	}
	if r.Gateway != nil {
		m.Gateway = r.Gateway
	}
	if r.BillingDay != nil {
		m.BillingDay = r.BillingDay
	}
	if r.AutoIsolate != nil {
		m.AutoIsolate = *r.AutoIsolate
	}
	if r.GracePeriodDays != nil {
		m.GracePeriodDays = r.GracePeriodDays
	}
	if r.Notes != nil {
		m.Notes = r.Notes
	}
}

// === RESPONSE ===

// PPPSecretInfo holds live MikroTik PPPSecret fields returned in responses.
// Password intentionally excluded.
type PPPSecretInfo struct {
	Service       string `json:"service,omitempty"`
	Profile       string `json:"profile,omitempty"`
	LocalAddress  string `json:"local_address,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"` // assigned IP
	Routes        string `json:"routes,omitempty"`
	LimitBytesIn  int64  `json:"limit_bytes_in,omitempty"`
	LimitBytesOut int64  `json:"limit_bytes_out,omitempty"`
	Comment       string `json:"comment,omitempty"`
	Disabled      bool   `json:"disabled"`
}

// SubscriptionResponse is the safe response struct.
// Password, MtPPPID, and DeletedAt are excluded.
type SubscriptionResponse struct {
	ID              string         `json:"id"`
	CustomerID      string         `json:"customer_id"`
	PlanID          string         `json:"plan_id"`
	RouterID        string         `json:"router_id"`
	Username        string         `json:"username"`
	StaticIP        *string        `json:"static_ip,omitempty"`
	Gateway         *string        `json:"gateway,omitempty"`
	Status          string         `json:"status"`
	ActivatedAt     *time.Time     `json:"activated_at,omitempty"`
	ExpiryDate      *time.Time     `json:"expiry_date,omitempty"`
	BillingDay      *int           `json:"billing_day,omitempty"`
	AutoIsolate     bool           `json:"auto_isolate"`
	GracePeriodDays *int           `json:"grace_period_days,omitempty"`
	SuspendReason   *string        `json:"suspend_reason,omitempty"`
	Notes           *string        `json:"notes,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Mikrotik        *PPPSecretInfo `json:"mikrotik,omitempty"` // nil when router unreachable
}

// === CONVERTERS ===

// SubscriptionToResponse converts a model + optional live MikroTik data to a response DTO.
// mt may be nil — Mikrotik field will be omitted from JSON.
func SubscriptionToResponse(m *model.Subscription, mt *mkdomain.PPPSecret) SubscriptionResponse {
	r := SubscriptionResponse{
		ID:              m.ID,
		CustomerID:      m.CustomerID,
		PlanID:          m.PlanID,
		RouterID:        m.RouterID,
		Username:        m.Username,
		StaticIP:        m.StaticIP,
		Gateway:         m.Gateway,
		Status:          m.Status,
		ActivatedAt:     m.ActivatedAt,
		ExpiryDate:      m.ExpiryDate,
		BillingDay:      m.BillingDay,
		AutoIsolate:     m.AutoIsolate,
		GracePeriodDays: m.GracePeriodDays,
		SuspendReason:   m.SuspendReason,
		Notes:           m.Notes,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if mt != nil {
		r.Mikrotik = &PPPSecretInfo{
			Service:       mt.Service,
			Profile:       mt.Profile,
			LocalAddress:  mt.LocalAddress,
			RemoteAddress: mt.RemoteAddress,
			Routes:        mt.Routes,
			LimitBytesIn:  mt.LimitBytesIn,
			LimitBytesOut: mt.LimitBytesOut,
			Comment:       mt.Comment,
			Disabled:      mt.Disabled,
		}
	}
	return r
}
