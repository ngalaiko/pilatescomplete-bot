package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dgraph-io/badger/v4"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/bookings"
	"github.com/pilatescomplete-bot/internal/calendars"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/devices"
	"github.com/pilatescomplete-bot/internal/events"
	"github.com/pilatescomplete-bot/internal/http/templates"
	"github.com/pilatescomplete-bot/internal/jobs"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

func Handler(
	logger *slog.Logger,
	renderer templates.Renderer,
	apiClient *pilatescomplete.APIClient,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
	authenticationService *authentication.Service,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
	calendarsService *calendars.Service,
) http.HandlerFunc {
	requireAuth := WithAuthentication(logger, authenticationService, credentialsStore)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", requireAuth(handleListEvents(logger, renderer, eventsService)))
	mux.HandleFunc("POST /{$}", handleLogin(logger, apiClient, credentialsStore, tokensStore))

	mux.HandleFunc("GET /login", handleAuthenticationPage(logger, renderer))

	mux.HandleFunc("POST /events/{event_id}/bookings", requireAuth(handleCreateBooking(logger, renderer, apiClient, eventsService)))
	mux.HandleFunc("DELETE /events/{event_id}/bookings/{booking_id}", requireAuth(handleDeleteBooking(logger, renderer, eventsService, apiClient)))

	mux.HandleFunc("POST /events/{event_id}/jobs", requireAuth(handleCreateJob(logger, renderer, eventsService, scheduler)))
	mux.HandleFunc("DELETE /events/{event_id}/jobs/{job_id}", requireAuth(handleDeleteJob(logger, renderer, eventsService, scheduler)))

	mux.HandleFunc("GET /calendars/{calendar_id}/pilatescomplete.ics", handleGetCalendar(logger, calendarsService))
	mux.HandleFunc("POST /calendars", requireAuth(handleCreateCalendar(logger, calendarsService)))

	return WithAccessLogs(logger)(mux.ServeHTTP)
}

func handleGetCalendar(logger *slog.Logger, calendarsService *calendars.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		id := parts[2]

		w.Header().Set("Content-Type", "text/calendar")
		if err := calendarsService.WriteICal(r.Context(), w, id); err != nil {
			logger.Error("write calendar", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleCreateCalendar(
	logger *slog.Logger,
	calendarsService *calendars.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cal, err := calendarsService.CreateCalendar(r.Context())
		if err != nil {
			logger.Error("create calendar", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		origin, err := url.Parse(r.Header.Get("Origin"))
		if err != nil {
			logger.Error("parse origin", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("webcal://%s/calendars/%s/pilatescomplete.ics", origin.Host, cal.ID), http.StatusFound)
	}
}

func handleDeleteJob(
	logger *slog.Logger,
	renderer templates.Renderer,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]
		jobID := parts[4]

		job, err := scheduler.FindByID(r.Context(), jobID)
		if err != nil {
			logger.Error("find job by id", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if job.BookEvent != nil && job.BookEvent.EventID != eventID {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := scheduler.DeleteByID(r.Context(), jobID); err != nil {
			logger.Error("delete by id", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), job.BookEvent.EventID)
		if err != nil {
			logger.Error("get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		if err := renderer.RenderEvent(w, event); err != nil {
			logger.Error("render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleDeleteBooking(
	logger *slog.Logger,
	renderer templates.Renderer,
	eventsService *events.Service,
	apiClient *pilatescomplete.APIClient,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]
		bookingID := parts[4]

		if err := apiClient.CancelBooking(r.Context(), bookingID); err != nil {
			logger.Error("cancal booking", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), eventID)
		if err != nil {
			logger.Error("get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		if err := renderer.RenderEvent(w, event); err != nil {
			logger.Error("render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleCreateBooking(
	logger *slog.Logger,
	renderer templates.Renderer,
	apiClient *pilatescomplete.APIClient,
	eventsService *events.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]

		if err := r.ParseForm(); err != nil {
			logger.Error("parse form", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, err := apiClient.BookActivity(r.Context(), eventID); errors.Is(err, pilatescomplete.ErrActivityBookingTooEarly) {
			logger.Error("book activity", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), eventID)
		if err != nil {
			logger.Error("get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		if err := renderer.RenderEvent(w, event); err != nil {
			logger.Error("render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleCreateJob(
	logger *slog.Logger,
	renderer templates.Renderer,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]

		if err := r.ParseForm(); err != nil {
			logger.Error("parse form", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), eventID)
		if err != nil {
			logger.Error("get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		job, err := jobs.NewBookEventJob(r.Context(), eventID, event.BookableFrom)
		if err != nil {
			logger.Error("new book event job", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := scheduler.Schedule(r.Context(), job); err != nil {
			logger.Error("schedule", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		event.Booking = &bookings.Booking{
			ID:     job.ID,
			Status: bookings.BookingStatusJobScheduled,
		}

		if err := renderer.RenderEvent(w, event); err != nil {
			logger.Error("render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleAuthenticationPage(logger *slog.Logger, renderer templates.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := renderer.RenderLoginPage(w, templates.LoginData{}); err != nil {
			logger.Error("render login page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleListEvents(
	logger *slog.Logger,
	renderer templates.Renderer,
	eventsService *events.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := eventsService.ListEvents(r.Context())
		if err != nil {
			logger.Error("parse form", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := renderer.RenderEventsPage(w, templates.EventsData{
			Events: events,
		}); err != nil {
			logger.Error("render events page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleLogin(
	logger *slog.Logger,
	client *pilatescomplete.APIClient,
	credentialsStore *credentials.Store,
	tokensStore *tokens.Store,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := tokens.FromContext(r.Context())
		if !isAuthenticated {
			if err := r.ParseForm(); err != nil {
				logger.Error("perse form", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			login, password := r.PostForm.Get("login"), r.PostForm.Get("password")

			cookie, err := client.Login(r.Context(), pilatescomplete.LoginData{
				Login:    login,
				Password: password,
			})
			if err != nil {
				logger.Error("login", "error", err)
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
					logger.Error("insert credentials", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			if err := tokensStore.Insert(r.Context(), &tokens.Token{
				CredentialsID: creds.ID,
				Token:         cookie.Value,
				Expires:       cookie.Expires,
			}); err != nil {
				logger.Error("insert token", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			device := devices.Device{
				CredentialsID: creds.ID,
			}

			for _, cookie := range device.ToCookies(r.TLS != nil) {
				w.Header().Add("Set-Cookie", cookie.String())
			}
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
