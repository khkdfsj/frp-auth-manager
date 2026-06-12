package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const serviceName = "frpc-agent"

type Config struct {
	ListenAddr           string `json:"listen_addr"`
	SharedSecret         string `json:"shared_secret"`
	FrpcExe              string `json:"frpc_exe"`
	BaseConfig           string `json:"base_config"`
	GeneratedConfig      string `json:"generated_config"`
	BackupDir            string `json:"backup_dir"`
	FrpcAdminAddr        string `json:"frpc_admin_addr"`
	FrpcAdminUser        string `json:"frpc_admin_user"`
	FrpcAdminPassword    string `json:"frpc_admin_password"`
	ManagementRemotePort int    `json:"management_remote_port"`
}

type SSHService struct {
	Name       string `json:"name"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
	RemotePort int    `json:"remote_port"`
	Enabled    bool   `json:"enabled"`
}

type ApplyRequest struct {
	Version     int64        `json:"version"`
	Services    []SSHService `json:"services"`
	GeneratedAt time.Time    `json:"generated_at"`
}

type ApplyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Version int64  `json:"version,omitempty"`
}

type Status struct {
	Online        bool      `json:"online"`
	Agent         string    `json:"agent"`
	FrpcRunning   bool      `json:"frpc_running"`
	ConfigVersion int64     `json:"config_version"`
	LastApplyOK   bool      `json:"last_apply_ok"`
	LastError     string    `json:"last_error,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Agent struct {
	cfg           Config
	mu            sync.Mutex
	configVersion int64
	lastApplyOK   bool
	lastError     string
	updatedAt     time.Time
	server        *http.Server
}

func main() {
	configPath := flag.String("config", "agent.json", "path to agent json config")
	flag.Parse()

	if handled, err := maybeRunWindowsService(*configPath); handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := run(context.Background(), *configPath); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, configPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	agent := &Agent{cfg: cfg, lastApplyOK: true, updatedAt: time.Now()}
	if err := agent.ensureFrpcRunning(); err != nil {
		log.Printf("ensure frpc running failed: %v", err)
		agent.lastApplyOK = false
		agent.lastError = err.Error()
	}
	return agent.serve(ctx)
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	base := filepath.Dir(path)
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:6700"
	}
	cfg.FrpcExe = abs(cfg.FrpcExe)
	cfg.BaseConfig = abs(cfg.BaseConfig)
	cfg.GeneratedConfig = abs(cfg.GeneratedConfig)
	cfg.BackupDir = abs(cfg.BackupDir)
	if cfg.GeneratedConfig == "" {
		cfg.GeneratedConfig = filepath.Join(base, "frpc.generated.toml")
	}
	if cfg.BackupDir == "" {
		cfg.BackupDir = filepath.Join(base, "backups")
	}
	if cfg.FrpcAdminAddr == "" {
		cfg.FrpcAdminAddr = "127.0.0.1:7400"
	}
	if cfg.FrpcAdminUser == "" {
		cfg.FrpcAdminUser = "admin"
	}
	if cfg.ManagementRemotePort == 0 {
		cfg.ManagementRemotePort = 6999
	}
	if cfg.FrpcExe == "" || cfg.BaseConfig == "" || cfg.GeneratedConfig == "" {
		return cfg, errors.New("frpc_exe, base_config and generated_config are required")
	}
	if cfg.SharedSecret == "" {
		return cfg, errors.New("shared_secret is required")
	}
	if cfg.FrpcAdminPassword == "" {
		return cfg, errors.New("frpc_admin_password is required")
	}
	return cfg, nil
}

func (a *Agent) serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", a.handleStatus)
	mux.HandleFunc("POST /v1/apply", a.handleApply)
	mux.HandleFunc("POST /v1/restart-frpc", a.handleRestart)

	a.server = &http.Server{
		Addr:         a.cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()
	log.Printf("frpc-agent listening on %s", a.cfg.ListenAddr)
	err := a.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (a *Agent) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !a.verifyRequest(w, r, nil) {
		return
	}
	a.mu.Lock()
	status := Status{
		Online:        true,
		Agent:         "frpc-agent",
		FrpcRunning:   isFrpcRunning(a.cfg.FrpcExe),
		ConfigVersion: a.configVersion,
		LastApplyOK:   a.lastApplyOK,
		LastError:     a.lastError,
		UpdatedAt:     a.updatedAt,
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

func (a *Agent) handleApply(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if !a.verifyRequest(w, r, body) {
		return
	}
	var req ApplyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ApplyResponse{Success: false, Message: "invalid json"})
		return
	}
	if err := a.validateApplyRequest(req); err != nil {
		a.recordApply(req.Version, false, err.Error())
		writeJSON(w, http.StatusBadRequest, ApplyResponse{Success: false, Message: err.Error(), Version: req.Version})
		return
	}
	if err := a.apply(req); err != nil {
		a.recordApply(req.Version, false, err.Error())
		writeJSON(w, http.StatusInternalServerError, ApplyResponse{Success: false, Message: err.Error(), Version: req.Version})
		return
	}
	a.recordApply(req.Version, true, "")
	writeJSON(w, http.StatusOK, ApplyResponse{Success: true, Message: "applied", Version: req.Version})
}

func (a *Agent) handleRestart(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if !a.verifyRequest(w, r, body) {
		return
	}
	if err := a.restartFrpc(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "frpc restarted"})
}

func (a *Agent) verifyRequest(w http.ResponseWriter, r *http.Request, body []byte) bool {
	if a.cfg.SharedSecret == "" {
		return true
	}
	ts := r.Header.Get("X-Agent-Timestamp")
	sig := r.Header.Get("X-Agent-Signature")
	if ts == "" || sig == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing signature"})
		return false
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Since(time.Unix(sec, 0)) > 5*time.Minute || time.Until(time.Unix(sec, 0)) > 5*time.Minute {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid timestamp"})
		return false
	}
	mac := hmac.New(sha256.New, []byte(a.cfg.SharedSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return false
	}
	return true
}

func (a *Agent) validateApplyRequest(req ApplyRequest) error {
	seen := map[int]bool{a.cfg.ManagementRemotePort: true}
	for _, svc := range req.Services {
		if strings.TrimSpace(svc.Name) == "" {
			return errors.New("service name is required")
		}
		if net.ParseIP(svc.TargetIP) == nil {
			return fmt.Errorf("invalid target ip: %s", svc.TargetIP)
		}
		if svc.TargetPort != 22 {
			return fmt.Errorf("only target port 22 is supported for %s", svc.Name)
		}
		if svc.RemotePort < 6222 || svc.RemotePort > 6299 {
			return fmt.Errorf("remote port out of range: %d", svc.RemotePort)
		}
		if seen[svc.RemotePort] {
			return fmt.Errorf("duplicate or reserved remote port: %d", svc.RemotePort)
		}
		seen[svc.RemotePort] = true
	}
	return nil
}

func (a *Agent) apply(req ApplyRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	generated, err := a.renderConfig(req)
	if err != nil {
		return err
	}
	tmp := a.cfg.GeneratedConfig + ".tmp"
	if err := os.WriteFile(tmp, []byte(generated), 0600); err != nil {
		return err
	}
	if err := a.verifyFrpcConfig(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.MkdirAll(a.cfg.BackupDir, 0700); err != nil {
		return err
	}
	if _, err := os.Stat(a.cfg.GeneratedConfig); err == nil {
		backup := filepath.Join(a.cfg.BackupDir, fmt.Sprintf("frpc.generated.%s.toml", time.Now().Format("20060102-150405")))
		_ = copyFile(a.cfg.GeneratedConfig, backup)
	}
	if err := os.Rename(tmp, a.cfg.GeneratedConfig); err != nil {
		return err
	}
	if err := a.reloadFrpc(); err == nil {
		return nil
	}
	return a.restartFrpc()
}

func (a *Agent) renderConfig(req ApplyRequest) (string, error) {
	base, err := os.ReadFile(a.cfg.BaseConfig)
	if err != nil {
		return "", err
	}
	baseGlobal, baseProxies := splitBaseConfig(base)
	var buf bytes.Buffer
	buf.Write(bytes.TrimSpace(baseGlobal))
	buf.WriteString("\n\n")
	buf.WriteString("# Generated by frpc-agent. Do not edit this file directly.\n")
	buf.WriteString(fmt.Sprintf("# version = %d generated_at = %s\n\n", req.Version, time.Now().Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("webServer.addr = %q\n", hostPart(a.cfg.FrpcAdminAddr)))
	buf.WriteString(fmt.Sprintf("webServer.port = %d\n", portPart(a.cfg.FrpcAdminAddr, 7400)))
	buf.WriteString(fmt.Sprintf("webServer.user = %q\n", a.cfg.FrpcAdminUser))
	buf.WriteString(fmt.Sprintf("webServer.password = %q\n\n", a.cfg.FrpcAdminPassword))
	if trimmed := bytes.TrimSpace(baseProxies); len(trimmed) > 0 {
		buf.Write(trimmed)
		buf.WriteString("\n\n")
	}
	buf.WriteString("[[proxies]]\n")
	buf.WriteString("name = \"frpc-agent-management\"\n")
	buf.WriteString("type = \"tcp\"\n")
	buf.WriteString("localIP = \"127.0.0.1\"\n")
	buf.WriteString(fmt.Sprintf("localPort = %d\n", portPart(a.cfg.ListenAddr, 6700)))
	buf.WriteString(fmt.Sprintf("remotePort = %d\n\n", a.cfg.ManagementRemotePort))

	sort.Slice(req.Services, func(i, j int) bool { return req.Services[i].RemotePort < req.Services[j].RemotePort })
	for _, svc := range req.Services {
		if !svc.Enabled {
			continue
		}
		buf.WriteString("[[proxies]]\n")
		buf.WriteString(fmt.Sprintf("name = %q\n", svc.Name))
		buf.WriteString("type = \"tcp\"\n")
		buf.WriteString(fmt.Sprintf("localIP = %q\n", svc.TargetIP))
		buf.WriteString("localPort = 22\n")
		buf.WriteString(fmt.Sprintf("remotePort = %d\n\n", svc.RemotePort))
	}
	return buf.String(), nil
}

func splitBaseConfig(base []byte) ([]byte, []byte) {
	marker := []byte("[[proxies]]")
	idx := bytes.Index(base, marker)
	if idx == -1 {
		return base, nil
	}
	return base[:idx], base[idx:]
}

func (a *Agent) verifyFrpcConfig(path string) error {
	cmd := exec.Command(a.cfg.FrpcExe, "verify", "-c", path)
	cmd.Dir = filepath.Dir(a.cfg.FrpcExe)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("frpc verify failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *Agent) reloadFrpc() error {
	cmd := exec.Command(a.cfg.FrpcExe, "reload", "-c", a.cfg.GeneratedConfig)
	cmd.Dir = filepath.Dir(a.cfg.FrpcExe)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("frpc reload failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *Agent) restartFrpc() error {
	_ = stopFrpc(a.cfg.FrpcExe)
	time.Sleep(2 * time.Second)
	return startFrpc(a.cfg.FrpcExe, a.cfg.GeneratedConfig)
}

func (a *Agent) ensureFrpcRunning() error {
	if isFrpcRunning(a.cfg.FrpcExe) {
		return nil
	}
	if _, err := os.Stat(a.cfg.GeneratedConfig); err != nil {
		return nil
	}
	return startFrpc(a.cfg.FrpcExe, a.cfg.GeneratedConfig)
}

func (a *Agent) recordApply(version int64, ok bool, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.configVersion = version
	a.lastApplyOK = ok
	a.lastError = message
	a.updatedAt = time.Now()
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func hostPart(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return "127.0.0.1"
	}
	return host
}

func portPart(addr string, fallback int) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return fallback
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return fallback
	}
	return n
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
