package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"frp_auth/database"
	"frp_auth/models"
)

var frpcAgentURL = getEnvOrDefault("FRPC_AGENT_URL", "http://127.0.0.1:6999")
var frpcAgentSecret = getEnvOrDefault("FRPC_AGENT_SECRET", "")

func ListSSHServices(w http.ResponseWriter, r *http.Request) {
	services, err := database.ListSSHServices()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if services == nil {
		services = []models.SSHService{}
	}
	writeJSON(w, http.StatusOK, services)
}

func CreateSSHService(w http.ResponseWriter, r *http.Request) {
	var req models.SSHServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := normalizeSSHServiceRequest(&req, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	svc, err := database.CreateSSHService(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	applyErr := applySSHServicesToAgent()
	if applyErr != nil {
		writeJSON(w, http.StatusCreated, map[string]interface{}{"service": svc, "apply_error": applyErr.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"service": svc, "applied": true})
}

func UpdateSSHService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid service id"})
		return
	}
	var req models.SSHServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := normalizeSSHServiceRequest(&req, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	svc, err := database.UpdateSSHService(id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	applyErr := applySSHServicesToAgent()
	if applyErr != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"service": svc, "apply_error": applyErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"service": svc, "applied": true})
}

func DeleteSSHService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid service id"})
		return
	}
	if err := database.DeleteSSHService(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	applyErr := applySSHServicesToAgent()
	if applyErr != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "apply_error": applyErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "applied": true})
}

func ApplySSHServices(w http.ResponseWriter, r *http.Request) {
	if err := applySSHServicesToAgent(); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "applied"})
}

func FrpcAgentStatus(w http.ResponseWriter, r *http.Request) {
	body, status, err := callFrpcAgent("GET", "/v1/status", nil)
	if err != nil {
		writeJSON(w, http.StatusOK, models.AgentStatus{
			Online:      false,
			LastApplyOK: false,
			LastError:   err.Error(),
			UpdatedAt:   time.Now(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func normalizeSSHServiceRequest(req *models.SSHServiceRequest, requireAll bool) error {
	req.Name = strings.TrimSpace(req.Name)
	req.TargetIP = strings.TrimSpace(req.TargetIP)
	req.Notes = strings.TrimSpace(req.Notes)
	if requireAll && req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if requireAll && req.TargetIP == "" {
		return fmt.Errorf("target_ip is required")
	}
	if req.TargetIP != "" && net.ParseIP(req.TargetIP) == nil {
		return fmt.Errorf("target_ip is invalid")
	}
	if requireAll && req.RemotePort == 0 {
		port, err := database.NextSSHRemotePort()
		if err != nil {
			return err
		}
		req.RemotePort = port
	}
	if req.RemotePort != 0 && (req.RemotePort < 6222 || req.RemotePort > 6299 || req.RemotePort == 6999) {
		return fmt.Errorf("remote_port must be in 6222-6299 and cannot be 6999")
	}
	return nil
}

func applySSHServicesToAgent() error {
	services, err := database.ListSSHServices()
	if err != nil {
		return err
	}
	payload := models.AgentApplyRequest{
		Version:     time.Now().Unix(),
		GeneratedAt: time.Now(),
		Services:    make([]models.AgentSSHService, 0, len(services)),
	}
	for _, svc := range services {
		payload.Services = append(payload.Services, models.AgentSSHService{
			Name:       svc.Name,
			TargetIP:   svc.TargetIP,
			TargetPort: 22,
			RemotePort: svc.RemotePort,
			Enabled:    svc.IsActive,
		})
	}
	body, status, err := callFrpcAgent("POST", "/v1/apply", payload)
	if err != nil {
		database.MarkSSHServicesApplyFailed(err.Error())
		return err
	}
	if status < 200 || status >= 300 {
		err := fmt.Errorf("agent apply failed: status %d: %s", status, strings.TrimSpace(string(body)))
		database.MarkSSHServicesApplyFailed(err.Error())
		return err
	}
	if err := database.MarkSSHServicesApplied(); err != nil {
		return err
	}
	return nil
}

func callFrpcAgent(method, path string, payload interface{}) ([]byte, int, error) {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequest(method, strings.TrimRight(frpcAgentURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	signAgentRequest(req, body)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody := new(bytes.Buffer)
	_, _ = respBody.ReadFrom(resp.Body)
	return respBody.Bytes(), resp.StatusCode, nil
}

func signAgentRequest(req *http.Request, body []byte) {
	if frpcAgentSecret == "" {
		return
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(frpcAgentSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write(body)
	req.Header.Set("X-Agent-Timestamp", ts)
	req.Header.Set("X-Agent-Signature", hex.EncodeToString(mac.Sum(nil)))
}
