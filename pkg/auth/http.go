package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	AuthEnabled bool   `json:"authEnabled"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"code": code, "message": msg})
}

func bearerFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// clientIP uses the TCP peer address only. X-Forwarded-For is not trusted
// because it is client-controlled and would let attackers evade rate limits.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RegisterHTTP registers POST /auth/login, POST /auth/logout and GET /auth/me
// on the grpc-gateway mux. Safe to call with auth disabled: login/logout then
// return 404-like errors and /auth/me reports authEnabled=false.
func (s *Service) RegisterHTTP(mux *runtime.ServeMux) error {
	if err := mux.HandlePath(http.MethodPost, "/auth/login", s.handleLogin); err != nil {
		return err
	}
	if err := mux.HandlePath(http.MethodPost, "/auth/logout", s.handleLogout); err != nil {
		return err
	}
	return mux.HandlePath(http.MethodGet, "/auth/me", s.handleMe)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if !s.Enabled() {
		writeErr(w, http.StatusNotFound, "authentication is not enabled")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password are required")
		return
	}
	res, err := s.Login(r.Context(), req.Username, req.Password, clientIP(r))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expiresAt"`
			Username  string `json:"username"`
			Role      string `json:"role"`
		}{res.Token, res.ExpiresAt.Format(time.RFC3339), res.Username, res.Role})
	case errors.Is(err, ErrInvalidCredentials):
		writeErr(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
	case errors.Is(err, ErrTooManyAttempts):
		w.Header().Set("Retry-After", "900")
		writeErr(w, http.StatusTooManyRequests, ErrTooManyAttempts.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "login failed")
	}
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if !s.Enabled() {
		writeErr(w, http.StatusNotFound, "authentication is not enabled")
		return
	}
	token := bearerFromRequest(r)
	if _, err := s.Authenticate(r.Context(), token); err != nil {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := s.Logout(r.Context(), token); err != nil {
		writeErr(w, http.StatusInternalServerError, "logout failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if !s.Enabled() {
		writeJSON(w, http.StatusOK, meResponse{Role: RoleAdmin, AuthEnabled: false})
		return
	}
	sess, err := s.Authenticate(r.Context(), bearerFromRequest(r))
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			writeErr(w, http.StatusUnauthorized, "authentication required")
		} else {
			writeErr(w, http.StatusServiceUnavailable, "unable to verify session")
		}
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Username: sess.Username, Role: sess.Role, AuthEnabled: true})
}
