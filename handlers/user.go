package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"frp_auth/database"
	"frp_auth/models"
)

// UserActivate handles the user-facing endpoint to activate their IP for a specific port.
// User provides their token and target port, gets temporary IP whitelist.
func UserActivate(w http.ResponseWriter, r *http.Request) {
	var req models.UserActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Token == "" || req.Port == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token and port are required"})
		return
	}

	// Get client IP
	clientIP := getClientIP(r)

	// Validate token
	token, err := database.GetTokenByValue(req.Token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.UserActivateResponse{
			Success: false, Message: "invalid token", IP: clientIP, Port: req.Port,
		})
		return
	}

	if !token.IsActive {
		writeJSON(w, http.StatusForbidden, models.UserActivateResponse{
			Success: false, Message: "token is disabled", IP: clientIP, Port: req.Port,
		})
		return
	}

	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		writeJSON(w, http.StatusForbidden, models.UserActivateResponse{
			Success: false, Message: "token has expired", IP: clientIP, Port: req.Port,
		})
		return
	}

	// Check port permission
	hasPerm, err := database.TokenHasPortPermission(token.ID, req.Port)
	if err != nil || !hasPerm {
		writeJSON(w, http.StatusForbidden, models.UserActivateResponse{
			Success: false, Message: "token does not have permission for this port", IP: clientIP, Port: req.Port,
		})
		return
	}

	// Add to IP whitelist (valid for 5 minutes)
	expiresAt := time.Now().Add(5 * time.Minute)
	if err := database.AddIPWhitelist(token.ID, clientIP, req.Port, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, models.UserActivateResponse{
		Success:   true,
		Message:   "IP authorized for this port",
		IP:        clientIP,
		Port:      req.Port,
		ExpiresAt: expiresAt,
	})
}

// AuthCheck is called by FRP to verify if an IP is authorized to use a port
func AuthCheck(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	portStr := r.URL.Query().Get("port")

	if ip == "" || portStr == "" {
		writeJSON(w, http.StatusBadRequest, models.AuthCheckResponse{
			Authorized: false, Message: "ip and port query params required",
		})
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.AuthCheckResponse{
			Authorized: false, Message: "invalid port",
		})
		return
	}

	// Check if this port requires auth
	portConfig, err := database.GetPortConfig(port)
	if err != nil {
		// If no config exists, default to no auth required (backward compatible)
		writeJSON(w, http.StatusOK, models.AuthCheckResponse{Authorized: true})
		return
	}

	if !portConfig.RequireAuth {
		writeJSON(w, http.StatusOK, models.AuthCheckResponse{Authorized: true})
		return
	}

	// Check IP whitelist
	authorized, err := database.CheckIPAuthorized(ip, port)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.AuthCheckResponse{
			Authorized: false, Message: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, models.AuthCheckResponse{
		Authorized: authorized,
		Message:    map[bool]string{true: "authorized", false: "unauthorized"}[authorized],
	})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
