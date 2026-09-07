package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const tokenTTL = 7 * 24 * time.Hour

var errInvalidToken = errors.New("invalid or expired token")

// newToken creates "<expires_unix>.<hmac_sig>" signed with the secret key.
func newToken(secret string) (string, time.Time) {
	expires := time.Now().Add(tokenTTL)
	payload := strconv.FormatInt(expires.Unix(), 10)
	return payload + "." + signPayload(payload, secret), expires
}

// validToken checks signature and expiry of a bearer token.
func validToken(value, secret string) bool {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	if !hmac.Equal([]byte(sig), []byte(signPayload(payload, secret))) {
		return false
	}
	expires, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return false
	}
	return true
}

func signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// bearerToken extracts the token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok && token != "" {
		return strings.TrimSpace(token), true
	}
	return "", false
}

// requireAuth wraps a JSON handler, rejecting requests without a valid bearer token.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok || !validToken(token, s.cfg.SecretKey) {
			writeJSONError(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		next(w, r)
	}
}
