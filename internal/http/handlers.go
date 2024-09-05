package http

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/device"
	"github.com/pilatescomplete-bot/internal/events"
	"github.com/pilatescomplete-bot/internal/http/templates"
	"github.com/pilatescomplete-bot/internal/jobs"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

func Handler(
	renderer templates.Renderer,
	apiClient *pilatescomplete.APIClient,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
	authenticationService *authentication.Service,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
) http.HandlerFunc {
	requireAuth := WithAuthentication(authenticationService, credentialsStore)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", requireAuth(handleListEvents(renderer, eventsService)))
	mux.HandleFunc("POST /", handleLogin(apiClient, credentialsStore, tokensStore))
	mux.HandleFunc("GET /login", handleAuthenticationPage(renderer))
	mux.HandleFunc("POST /events/{event_id}/bookings", requireAuth(handleCreateBooking(apiClient, scheduler)))
	mux.HandleFunc("POST /events/{event_id}/bookings/{booking_id}", requireAuth(handleDeleteBooking(apiClient)))
	mux.HandleFunc("POST /jobs/{job_id}", requireAuth(handleDeleteJob(scheduler)))
	return mux.ServeHTTP
}

func handleDeleteJob(scheduler *jobs.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		bookingID := parts[2]
		isDelete := r.URL.Query().Get("delete") == "true"

		if isDelete {
			if err := scheduler.DeleteByID(r.Context(), bookingID); err != nil {
				log.Printf("[ERROR] %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, r.Referer(), http.StatusFound)
	}
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

func handleCreateBooking(
	apiClient *pilatescomplete.APIClient,
	scheduler *jobs.Scheduler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]

		if err := r.ParseForm(); err != nil {
			log.Printf("[ERROR] parse form: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, err := apiClient.BookActivity(r.Context(), eventID); errors.Is(err, pilatescomplete.ErrActivityBookingTooEarly) {
			bookableFrom, err := time.Parse(time.RFC3339, r.PostForm.Get("bookable_from"))
			if err != nil {
				log.Printf("[ERROR] parse bookable_from: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			job, err := jobs.NewBookEventJob(r.Context(), eventID, bookableFrom)
			if err != nil {
				log.Printf("[ERROR] new book event job: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := scheduler.Schedule(r.Context(), job); err != nil {
				log.Printf("[ERROR] schedule: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		} else if err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, r.Referer(), http.StatusFound)
	}
}

func handleAuthenticationPage(renderer templates.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := renderer.RenderLoginPage(w, templates.LoginData{}); err != nil {
			log.Printf("[ERROR] %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleListEvents(
	renderer templates.Renderer,
	eventsService *events.Service,
) http.HandlerFunc {
	parseDateOrNow := func(date string) time.Time {
		t, err := time.Parse(time.DateOnly, date)
		if err != nil {
			return time.Now()
		}
		return t
	}
	return func(w http.ResponseWriter, r *http.Request) {
		from := parseDateOrNow(r.URL.Query().Get("from"))
		to := parseDateOrNow(r.URL.Query().Get("to"))
		if to.Before(from) {
			to = from
		}
		events, err := eventsService.ListEvents(r.Context(), events.ListEventsInput{
			From: &from,
			To:   &to,
		})
		if err != nil {
			log.Printf("[ERROR] parse form: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := renderer.RenderEventsPage(w, templates.EventsData{
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
				log.Printf("[ERROR] login: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			creds, err := credentialsStore.FindByLogin(r.Context(), login)
			if errors.Is(err, badger.ErrKeyNotFound) {
				creds = &credentials.Credentials{
					ID:       gonanoid.Must(),
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
