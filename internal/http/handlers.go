package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/device"
	"github.com/pilatescompletebot/internal/http/templates"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
)

func Handler(
	client *pilatescomplete.Client,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
) http.HandlerFunc {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndexPage())
	mux.HandleFunc("POST /", handleLogin(client, credentialsStore, tokensStore))
	return WithMiddlewares(
		WithAuthentication(credentialsStore),
		WithToken(client, tokensStore, credentialsStore),
	)(mux.ServeHTTP)
}

func handleAuthenticationPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := templates.Login(w, templates.LoginData{}); err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleIndexPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := tokens.FromContext(r.Context())
		if !isAuthenticated {
			handleAuthenticationPage()(w, r)
			return
		}
		if err := templates.Index(w, templates.IndexData{}); err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleLogin(
	client *pilatescomplete.Client,
	credentialsStore *credentials.Store,
	tokensStore *tokens.Store,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := tokens.FromContext(r.Context())
		if !isAuthenticated {
			if err := r.ParseForm(); err != nil {
				log.Printf("[ERROR] parse form: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			login, password := r.PostForm.Get("login"), r.PostForm.Get("password")

			cookie, err := client.Login(pilatescomplete.LoginData{
				Login:    login,
				Password: password,
			})
			if err != nil {
				// TODO: handle in a good way
				log.Printf("[ERROR] login: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			creds, err := credentialsStore.FindByLogin(r.Context(), login)
			if errors.Is(err, badger.ErrKeyNotFound) {
				creds = &credentials.Credentials{
					ID:       credentials.NewID(),
					Login:    login,
					Password: password,
				}
				if err := credentialsStore.Insert(r.Context(), creds); err != nil {
					log.Printf("[ERROR] insert credentials: %s", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			if err := tokensStore.Insert(r.Context(), &tokens.Token{
				CredentialsID: creds.ID,
				Token:         cookie.Value,
				Expires:       cookie.Expires,
			}); err != nil {
				log.Printf("[ERROR] insert token: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			device := device.Device{
				CredentialsID: creds.ID,
			}

			for _, cookie := range device.ToCookies(r.TLS != nil) {
				w.Header().Add("Set-Cookie", cookie.String())
			}
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
