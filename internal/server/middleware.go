package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	accountContextKey  contextKey = "account"
	defaultMaxBodySize            = 4 << 20
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		start := time.Now()
		next.ServeHTTP(rec, r)

		event := accessLogEvent(r.URL.Path, rec.statusCode)
		event.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rec.statusCode).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Dur("duration", time.Since(start)).
			Msg("http request completed")
	})
}

func AuthMiddleware(manager *account.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if manager == nil {
				writeAPIError(w, http.StatusInternalServerError, "server misconfigured", "internal_error", "internal_error")
				return
			}

			token, ok := parseBearerToken(r.Header.Get("Authorization"))
			if !ok {
				log.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("request rejected: missing or invalid bearer token")
				writeAPIError(w, http.StatusUnauthorized, "missing or invalid bearer token", "invalid_request_error", "invalid_api_key")
				return
			}

			acct, err := manager.Get(token)
			if err != nil {
				log.Warn().
					Err(err).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("account_token", maskToken(token)).
					Msg("request rejected: account lookup failed")
				writeAPIError(w, http.StatusUnauthorized, "invalid account token", "invalid_request_error", "invalid_api_key")
				return
			}

			log.Debug().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("account_uuid", acct.UUID).
				Msg("request authenticated")

			ctx := context.WithValue(r.Context(), accountContextKey, acct)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestSizeLimitMiddleware(max int64) func(http.Handler) http.Handler {
	if max <= 0 {
		max = defaultMaxBodySize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseBearerToken(header string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func accountFromContext(ctx context.Context) (*account.Account, bool) {
	acct, ok := ctx.Value(accountContextKey).(*account.Account)
	if !ok || acct == nil {
		return nil, false
	}
	return acct, true
}

func accessLogEvent(path string, statusCode int) *zerolog.Event {
	switch {
	case statusCode >= http.StatusInternalServerError:
		return log.Error()
	case statusCode >= http.StatusBadRequest:
		return log.Warn()
	case path == "/health":
		return log.Debug()
	default:
		return log.Info()
	}
}

func maskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return token
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, statusCode int, message, errType, code string) {
	writeJSON(w, statusCode, map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    errType,
			"code":    code,
		},
	})
}
