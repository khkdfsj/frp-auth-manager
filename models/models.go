package models

import "time"

type AdminUser struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Token struct {
	ID           int        `json:"id"`
	Token        string     `json:"token"`
	Name         string     `json:"name"`
	Notes        string     `json:"notes"`
	CreatedBy    int        `json:"created_by"`
	IsActive     bool       `json:"is_active"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	TrafficLimit int64      `json:"traffic_limit"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PortPermission struct {
	ID        int       `json:"id"`
	TokenID   int       `json:"token_id"`
	Port      int       `json:"port"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type PortConfig struct {
	ID          int       `json:"id"`
	Port        int       `json:"port"`
	RequireAuth bool      `json:"require_auth"`
	CreatedAt   time.Time `json:"created_at"`
}

type IPWhitelist struct {
	ID        int       `json:"id"`
	TokenID   int       `json:"token_id"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

type CreateTokenRequest struct {
	Name        string     `json:"name"`
	Notes       string     `json:"notes,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	TrafficLimit int64     `json:"traffic_limit,omitempty"`
}

type UpdateTokenRequest struct {
	Name         *string    `json:"name,omitempty"`
	Notes        *string    `json:"notes,omitempty"`
	IsActive     *bool      `json:"is_active,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	TrafficLimit *int64     `json:"traffic_limit,omitempty"`
}

type PortPermissionRequest struct {
	TokenID int `json:"token_id"`
	Port    int `json:"port"`
}

type PortConfigRequest struct {
	Port        int  `json:"port"`
	RequireAuth bool `json:"require_auth"`
}

type UserActivateRequest struct {
	Token string `json:"token"`
	Port  int    `json:"port"`
}

type UserActivateResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AuthCheckResponse struct {
	Authorized bool   `json:"authorized"`
	Message    string `json:"message,omitempty"`
}
