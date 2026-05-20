package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"frp_auth/database"
	"frp_auth/middleware"
	"frp_auth/models"
)

func Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	admin, err := database.GetAdminByUsername(req.Username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}

	hash := sha256.Sum256([]byte(req.Password))
	if hex.EncodeToString(hash[:]) != admin.PasswordHash {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}

	sessionToken := middleware.CreateSession(admin.ID, admin.Username)
	writeJSON(w, http.StatusOK, models.LoginResponse{
		Token:    sessionToken,
		Username: admin.Username,
	})
}

func CheckSession(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"valid": "false"})
		return
	}
	session, ok := middleware.GetSession(parts[1])
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"valid": "false"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":    true,
		"username": session.Username,
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		middleware.DestroySession(parts[1])
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已退出登录"})
}

// Token management
func CreateToken(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "用户名不能为空"})
		return
	}

	b := make([]byte, 24)
	rand.Read(b)
	tokenValue := "frp_" + hex.EncodeToString(b)

	token, err := database.CreateToken(tokenValue, req.Name, req.Notes, 1, req.ExpiresAt, req.TrafficLimit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, token)
}

func ListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := database.ListTokens()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type TokenWithPerms struct {
		models.Token
		Permissions     []models.PortPermission `json:"permissions"`
		ActivationCount int                     `json:"activation_count"`
	}

	result := make([]TokenWithPerms, 0, len(tokens))
	for _, t := range tokens {
		perms, _ := database.GetTokenPortPermissions(t.ID)
		if perms == nil {
			perms = []models.PortPermission{}
		}
		count, _ := database.GetTokenActivationCount(t.ID)
		result = append(result, TokenWithPerms{Token: t, Permissions: perms, ActivationCount: count})
	}
	writeJSON(w, http.StatusOK, result)
}

func UpdateToken(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 Token ID"})
		return
	}

	var req models.UpdateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	// Build a set of set operations
	if req.Name != nil {
		database.UpdateToken(id, req.Name, nil, nil, nil, nil)
	}
	if req.Notes != nil {
		database.UpdateToken(id, nil, req.Notes, nil, nil, nil)
	}
	if req.IsActive != nil {
		database.UpdateToken(id, nil, nil, req.IsActive, nil, nil)
	}
	if req.ExpiresAt != nil {
		database.UpdateToken(id, nil, nil, nil, req.ExpiresAt, nil)
	}
	if req.TrafficLimit != nil {
		database.UpdateToken(id, nil, nil, nil, nil, req.TrafficLimit)
	}

	// Also handle port permissions if included
	var permReq struct {
		Permissions []models.PortPermissionRequest `json:"permissions,omitempty"`
	}
	// Re-decode to check for permissions field
	json.NewDecoder(r.Body).Decode(&permReq)
	_ = permReq // Handled separately

	writeJSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

func DeleteToken(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 Token ID"})
		return
	}

	if err := database.DeleteToken(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// Port permission management
func AddPortPermission(w http.ResponseWriter, r *http.Request) {
	var req models.PortPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	if err := database.AddPortPermission(req.TokenID, req.Port); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": fmt.Sprintf("端口 %d 权限已添加", req.Port)})
}

func RemovePortPermission(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的权限 ID"})
		return
	}

	if err := database.RemovePortPermission(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "权限已移除"})
}

func RemovePortPermissionByTokenPort(w http.ResponseWriter, r *http.Request) {
	tokenIDStr := r.URL.Query().Get("token_id")
	portStr := r.URL.Query().Get("port")
	tokenID, _ := strconv.Atoi(tokenIDStr)
	port, _ := strconv.Atoi(portStr)
	if tokenID == 0 || port == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "参数不完整"})
		return
	}
	if err := database.RemovePortPermissionByTokenPort(tokenID, port); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "权限已移除"})
}

func ListPortPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := database.GetAllPortPermissions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if perms == nil {
		perms = []models.PortPermission{}
	}
	writeJSON(w, http.StatusOK, perms)
}

// Port config management
func SetPortConfig(w http.ResponseWriter, r *http.Request) {
	var req models.PortConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.AuthMode == "" {
		req.AuthMode = "token"
	}
	if req.AuthMode != "open" && req.AuthMode != "token" && req.AuthMode != "ip" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth_mode 必须为 open、token 或 ip 之一"})
		return
	}

	if err := database.SetPortConfig(req.Port, req.AuthMode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("端口 %d 鉴权模式已设为：%s", req.Port, req.AuthMode)})
}

func ListPortConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := database.ListPortConfigs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if configs == nil {
		configs = []models.PortConfig{}
	}
	writeJSON(w, http.StatusOK, configs)
}

func DeletePortConfig(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的配置 ID"})
		return
	}

	configs, _ := database.ListPortConfigs()
	var port int
	for _, c := range configs {
		if c.ID == id {
			port = c.Port
			break
		}
	}
	if port == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "配置不存在"})
		return
	}

	if err := database.DeletePortConfig(port); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "配置已删除"})
}

// Port IP allowlist management
func AddPortIPAllow(w http.ResponseWriter, r *http.Request) {
	var req models.PortIPAllowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Port == 0 || req.IP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "端口和IP不能为空"})
		return
	}

	if err := database.AddPortIPAllow(req.Port, req.IP, req.Notes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": fmt.Sprintf("端口 %d 已允许 IP %s 访问", req.Port, req.IP)})
}

func RemovePortIPAllow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 ID"})
		return
	}

	if err := database.RemovePortIPAllow(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "IP 限制已移除"})
}

func ListPortIPAllowlist(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	var port int
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	entries, err := database.ListPortIPAllowlist(port)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []models.PortIPAllowEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
