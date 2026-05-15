package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

var frpDashboardBase = getEnvOrDefault("FRP_DASHBOARD_URL", "http://127.0.0.1:7501")
var frpDashboardUser = getEnvOrDefault("FRP_DASHBOARD_USER", "synydxxxzxdxswlxxzxyxbkhk983426@")
var frpDashboardPass = getEnvOrDefault("FRP_DASHBOARD_PASS", "88487016@983426@dfsjkhk@")

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func basicAuthHeader(user, pass string) string {
	auth := user + ":" + pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

func ProxyFRPAPI(w http.ResponseWriter, r *http.Request) {
	// Route: /api/frp/proxy/... -> forward to FRP dashboard
	subPath := r.URL.Path[len("/api/frp/"):]
	targetURL := fmt.Sprintf("%s/api/%s", frpDashboardBase, subPath)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create proxy request"})
		return
	}
	req.Header.Set("Authorization", basicAuthHeader(frpDashboardUser, frpDashboardPass))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("frp dashboard unreachable: %v", err)})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func proxyFRPStatic(w http.ResponseWriter, r *http.Request) {
	// Serve FRP static files (CSS, JS) from the dashboard
	subPath := r.URL.Path[len("/api/frp-static/"):]
	targetURL := fmt.Sprintf("%s/%s", frpDashboardBase, subPath)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// FRPServerInfo returns FRP server status
func FRPServerInfo(w http.ResponseWriter, r *http.Request) {
	targetURL := fmt.Sprintf("%s/api/serverinfo", frpDashboardBase)
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("Authorization", basicAuthHeader(frpDashboardUser, frpDashboardPass))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "frp unreachable"})
		return
	}
	defer resp.Body.Close()

	var data interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	writeJSON(w, http.StatusOK, data)
}
