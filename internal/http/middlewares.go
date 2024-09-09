package http

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/devices"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

func WithMiddlewares(middlewares ...Middleware) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			for i := len(middlewares) - 1; i > -1; i-- {
				next = middlewares[i](next)
			}
			next(w, r)
		}
	}
}

type responseWriter struct {
	statusCode int
	http.ResponseWriter
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func WithAccessLogs(logger *slog.Logger) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next(writer, r)

			logger.Info("request",
				"method", r.Method,
				"url", r.URL,
				"status_code", writer.statusCode,
				"duration", time.Since(start),
				"user_agent", r.Header.Get("User-Agent"),
			)
		}
	}
}

func WithAuthentication(
	logger *slog.Logger,
	authenticationService *authentication.Service,
	credentialsStore *credentials.Store,
) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			device, ok := devices.FromCookies(r.Cookies())
			if !ok {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			if _, err := credentialsStore.FindByID(r.Context(), device.CredentialsID); errors.Is(err, credentials.ErrNotFound) {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			} else if err != nil {
				logger.Error("find credentials", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else {
				r = r.WithContext(devices.NewContext(r.Context(), device))
				for _, cookie := range device.ToCookies(r.Header.Get("X-Forwarded-Proto") == "https") {
					w.Header().Add("Set-Cookie", cookie.String())
				}
			}

			ctx, err := authenticationService.AuthenticateContext(r.Context(), device.CredentialsID)
			if err != nil {
				logger.Error("authenticate context", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			next(w, r.WithContext(ctx))
		}
	}
}
