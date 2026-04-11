package service

import "errors"

// Sentinel errors for MikroTik PPP sync. Use errors.Is in HTTP handlers to map to 409, etc.
var (
	ErrMikrotikPPPProfileExists   = errors.New("ppp profile with this name already exists on the router")
	ErrMikrotikPPPProfileNotFound = errors.New("ppp profile not found on the router")
	ErrMikrotikPPPSecretExists    = errors.New("ppp secret with this username already exists on the router")
	ErrMikrotikPPPSecretNotFound  = errors.New("ppp secret not found on the router")
)
