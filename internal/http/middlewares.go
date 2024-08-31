package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/device"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
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

func WithToken(
	client *pilatescomplete.Client,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dvc, ok := device.FromContext(r.Context())
			if !ok {
				next(w, r)
				return
			}

			token, err := tokensStore.FindByID(r.Context(), dvc.CredentialsID)
			if errors.Is(err, tokens.ErrNotFound) {
				creds, err := credentialsStore.FindByID(r.Context(), dvc.CredentialsID)
				if err != nil {
					if errors.Is(err, credentials.ErrNotFound) {
						next(w, r)
						return
					}
					log.Printf("[ERROR] find credentials: %s", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				cookie, err := client.Login(pilatescomplete.LoginData{
					Login:    creds.Login,
					Password: creds.Password,
				})
				if err != nil {
					log.Printf("[ERROR] login: %s", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				token = &tokens.Token{
					CredentialsID: creds.ID,
					Token:         cookie.Value,
					Expires:       cookie.Expires,
				}

				if err := tokensStore.Insert(r.Context(), token); err != nil {
					log.Printf("[ERROR] insert token: %s", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else if err != nil {
				log.Printf("[ERROR] find token: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			next(w, r.WithContext(tokens.NewContext(r.Context(), token)))
		}
	}
}

func WithAuthentication(
	credentialsStore *credentials.Store,
) Middleware {
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
