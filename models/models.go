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
	ID         int       `json:"id"`
	Port       int       `json:"port"`
	AuthMode   string    `json:"auth_mode"`    // "open", "token", "ip"
	IPListMode string    `json:"ip_list_mode"` // "whitelist" or "blacklist"
	CreatedAt  time.Time `json:"created_at"`
}

// Backward-compat: derived from AuthMode
func (p PortConfig) RequireAuth() bool { return p.AuthMode != "open" }

type PortIPAllowEntry struct {
	ID        int       `json:"id"`
	Port      int       `json:"port"`
	IP        string    `json:"ip"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type PortIPAllowRequest struct {
	Port  int    `json:"port"`
	IP    string `json:"ip"`
	Notes string `json:"notes,omitempty"`
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
	Name         string     `json:"name"`
	Notes        string     `json:"notes,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	TrafficLimit int64      `json:"traffic_limit,omitempty"`
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
	Port       int    `json:"port"`
	AuthMode   string `json:"auth_mode"`    // "open", "token", "ip"
	IPListMode string `json:"ip_list_mode"` // "whitelist" or "blacklist"
}

type BatchPortIPRequest struct {
	Port  int      `json:"port"`
	IPs   []string `json:"ips"`
	Notes string   `json:"notes,omitempty"`
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

type SSHService struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	TargetIP    string     `json:"target_ip"`
	RemotePort  int        `json:"remote_port"`
	IsActive    bool       `json:"is_active"`
	Notes       string     `json:"notes"`
	ApplyStatus string     `json:"apply_status"`
	LastError   string     `json:"last_error"`
	LastApplied *time.Time `json:"last_applied,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type SSHServiceRequest struct {
	Name       string `json:"name"`
	TargetIP   string `json:"target_ip"`
	RemotePort int    `json:"remote_port,omitempty"`
	IsActive   *bool  `json:"is_active,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type AgentSSHService struct {
	Name       string `json:"name"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
	RemotePort int    `json:"remote_port"`
	Enabled    bool   `json:"enabled"`
}

type AgentApplyRequest struct {
	Version     int64             `json:"version"`
	Services    []AgentSSHService `json:"services"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type AgentApplyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Version int64  `json:"version,omitempty"`
}

type AgentStatus struct {
	Online        bool      `json:"online"`
	Agent         string    `json:"agent,omitempty"`
	FrpcRunning   bool      `json:"frpc_running"`
	ConfigVersion int64     `json:"config_version"`
	LastApplyOK   bool      `json:"last_apply_ok"`
	LastError     string    `json:"last_error,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}
