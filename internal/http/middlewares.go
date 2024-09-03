package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/device"
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

func WithToken(authenticationService *authentication.Service) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dvc, ok := device.FromContext(r.Context())
			if !ok {
				next(w, r)
				return
			}

			ctx, err := authenticationService.AuthenticateContext(r.Context(), dvc.CredentialsID)
			if errors.Is(err, credentials.ErrNotFound) {
				next(w, r)
				return
			} else if err != nil {
				log.Printf("[ERROR] find token: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			next(w, r.WithContext(ctx))
		}
	}
}

func WithAuthentication(credentialsStore *credentials.Store) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dvc, ok := device.FromCookies(r.Cookies())
			if !ok {
				next(w, r)
				return
			}

			if _, err := credentialsStore.FindByID(r.Context(), dvc.CredentialsID); errors.Is(err, credentials.ErrNotFound) {
				next(w, r)
				return
			} else if err != nil {
				log.Printf("[ERROR] find credentials: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else {
				r = r.WithContext(device.NewContext(r.Context(), dvc))
				for _, cookie := range dvc.ToCookies(r.TLS != nil) {
					w.Header().Add("Set-Cookie", cookie.String())
				}
			}
			next(w, r)
		}
	}
}
