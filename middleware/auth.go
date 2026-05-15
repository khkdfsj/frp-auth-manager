package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	adminSessions = make(map[string]sessionInfo)
	sessionMu     sync.RWMutex
)

type sessionInfo struct {
	AdminID   int
	Username  string
	ExpiresAt time.Time
}

func init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			sessionMu.Lock()
			now := time.Now()
			for k, v := range adminSessions {
				if v.ExpiresAt.Before(now) {
					delete(adminSessions, k)
				}
			}
			sessionMu.Unlock()
		}
	}()
}

func CreateSession(adminID int, username string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	sessionMu.Lock()
	adminSessions[token] = sessionInfo{
		AdminID:   adminID,
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	sessionMu.Unlock()
	return token
}

func DestroySession(token string) {
	sessionMu.Lock()
	delete(adminSessions, token)
	sessionMu.Unlock()
}

func GetSession(token string) (sessionInfo, bool) {
	sessionMu.RLock()
	s, ok := adminSessions[token]
	sessionMu.RUnlock()
	if !ok || s.ExpiresAt.Before(time.Now()) {
		return sessionInfo{}, false
	}
	return s, true
}

// AdminAuth middleware
func AdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		session, ok := GetSession(parts[1])
		if !ok {
			http.Error(w, `{"error":"invalid or expired session"}`, http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(r.Context()))
		_ = session
	}
}
