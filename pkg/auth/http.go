package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"github.com/hieunyit/tutti/pkg/dom"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	AuthEnabled bool   `json:"authEnabled"`
	// Source is "config" (read-only) or "managed".
	Source             string `json:"source,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
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

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// resolveClientIP returns the client address. X-Forwarded-For / X-Real-IP are
// honoured only when the TCP peer is a trusted proxy; XFF is walked from right
// to left and the first address that is not itself a trusted proxy wins.
func resolveClientIP(r *http.Request, trusted []*net.IPNet) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		peer = host
	}
	peerIP := net.ParseIP(peer)
	if len(trusted) == 0 || peerIP == nil || !ipInNets(peerIP, trusted) {
		return peer
	}
	if xff := strings.Join(r.Header.Values("X-Forwarded-For"), ","); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			p := strings.TrimSpace(parts[i])
			ip := net.ParseIP(p)
			if ip == nil {
				// malformed entry: stop, cannot be trusted further
				break
			}
			if !ipInNets(ip, trusted) {
				return ip.String()
			}
		}
	}
	if xr := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xr != nil {
		return xr.String()
	}
	return peer
}

func (s *Service) clientIP(r *http.Request) string { return resolveClientIP(r, s.trusted) }

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
	if err := mux.HandlePath(http.MethodPost, "/auth/password", s.handleChangePassword); err != nil {
		return err
	}
	return mux.HandlePath(http.MethodGet, "/auth/me", s.handleMe)
}

// handleChangePassword lets a signed-in managed user change its own password. Other sessions of the user are revoked.
func (s *Service) handleChangePassword(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if !s.Enabled() {
		writeErr(w, http.StatusConflict, "sign-in is disabled")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
		writeErr(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}
	err := s.ChangeOwnPassword(r.Context(), bearerFromRequest(r), req.CurrentPassword, req.NewPassword, s.clientIP(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrInvalidSession):
		writeErr(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, ErrInvalidCredentials):
		writeErr(w, http.StatusForbidden, "current password is incorrect")
	case errors.Is(err, ErrTooManyAttempts):
		w.Header().Set("Retry-After", "900")
		writeErr(w, http.StatusTooManyRequests, ErrTooManyAttempts.Error())
	case errors.Is(err, dom.ErrInvalidArg):
		writeErr(w, http.StatusBadRequest, trimErrPrefix(err, dom.ErrInvalidArg))
	case errors.Is(err, dom.ErrFailedPrecondition):
		writeErr(w, http.StatusConflict, trimErrPrefix(err, dom.ErrFailedPrecondition))
	default:
		writeErr(w, http.StatusInternalServerError, "password change failed")
	}
}

func trimErrPrefix(err, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
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
	res, err := s.Login(r.Context(), req.Username, req.Password, s.clientIP(r))
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
	writeJSON(w, http.StatusOK, meResponse{Username: sess.Username, Role: sess.Role, AuthEnabled: true, Source: sess.Source, MustChangePassword: sess.MustChangePassword})
}
