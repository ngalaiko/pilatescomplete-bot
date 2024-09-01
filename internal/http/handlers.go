package http

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/device"
	"github.com/pilatescompletebot/internal/events"
	"github.com/pilatescompletebot/internal/http/templates"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
)

func Handler(
	apiClient *pilatescomplete.APIClient,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
) http.HandlerFunc {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndexPage(apiClient))
	mux.HandleFunc("POST /", handleLogin(apiClient, credentialsStore, tokensStore))
	mux.HandleFunc("POST /events/{event_id}/bookings", handleCreateBooking(apiClient))
	mux.HandleFunc("POST /events/{event_id}/bookings/{booking_id}", handleDeleteBooking(apiClient))
	return WithMiddlewares(
		WithAuthentication(credentialsStore),
		WithToken(apiClient, tokensStore, credentialsStore),
	)(mux.ServeHTTP)
}

func handleDeleteBooking(apiClient *pilatescomplete.APIClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		bookingID := parts[4]
		isDelete := r.URL.Query().Get("delete") == "true"

		if isDelete {
			if err := apiClient.CancelBooking(r.Context(), bookingID); err != nil {
				log.Printf("[ERROR] %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, r.Referer(), http.StatusFound)
	}
}

func handleCreateBooking(apiClient *pilatescomplete.APIClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]

		if _, err := apiClient.BookActivity(r.Context(), eventID); errors.Is(err, pilatescomplete.ErrActivityBookingTooEarly) {
			// TODO handle scheduling
			fmt.Println("TOO EARLY")
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, r.Referer(), http.StatusFound)
	}
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

func handleIndexPage(client *pilatescomplete.APIClient) http.HandlerFunc {
	parseDateOrNow := func(date string) time.Time {
		t, err := time.Parse(time.DateOnly, date)
		if err != nil {
			return time.Now()
		}
		return t
	}

	return func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := tokens.FromContext(r.Context())
		if !isAuthenticated {
			handleAuthenticationPage()(w, r)
			return
		}

		from := parseDateOrNow(r.URL.Query().Get("from"))
		to := parseDateOrNow(r.URL.Query().Get("to"))
		if to.Before(from) {
			to = from
		}

		apiResponse, err := client.ListEvents(r.Context(), pilatescomplete.ListEventsInput{
			From: &from,
			To:   &to,
		})
		if err != nil {
			log.Printf("[ERROR] parse form: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		events, err := events.EventsFromAPI(apiResponse.Events)
		if err != nil {
			log.Printf("[ERROR] parse events: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := templates.Events(w, templates.EventsData{
			Events: events,
			From:   from,
			To:     to,
		}); err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleLogin(
	client *pilatescomplete.APIClient,
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

			cookie, err := client.Login(r.Context(), pilatescomplete.LoginData{
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
