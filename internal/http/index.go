package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/device"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/templates"
	"github.com/pilatescompletebot/internal/tokens"
)

func WithToken(
	client *pilatescomplete.Client,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dvc, ok := device.FromContext(r.Context())
		if !ok {
			next(w, r)
			return
		}

		token, err := tokensStore.FindByID(r.Context(), dvc.CredentialsID)
		if errors.Is(err, tokens.ErrNotFound) {
			creds, err := credentialsStore.FindByID(r.Context(), dvc.CredentialsID)
			if errors.Is(err, badger.ErrKeyNotFound) {
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
		}

		next(w, r.WithContext(tokens.NewContext(r.Context(), token)))
	}
}

func WithAuthentication(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dvc, ok := device.FromCookies(r.Cookies())
		if ok {
			r = r.WithContext(device.NewContext(r.Context(), dvc))
			for _, cookie := range dvc.ToCookies(r.TLS != nil) {
				w.Header().Add("Set-Cookie", cookie.String())
			}
		}
		next(w, r)
	}
}

func Handler(
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
) http.HandlerFunc {
	client := pilatescomplete.Client{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := device.FromContext(r.Context())
		if err := templates.Index(w, templates.IndexData{
			Authenticated: isAuthenticated,
		}); err != nil {
			log.Printf("[ERROR] /index.html: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := device.FromContext(r.Context())
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
	})
	return WithToken(&client, tokensStore, credentialsStore,
		WithAuthentication(
			mux.ServeHTTP,
		))
}
