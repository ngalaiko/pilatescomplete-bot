package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	"github.com/pilatescomplete-bot/internal/statistics"
	"github.com/pilatescomplete-bot/internal/tokens"
)

func Handler(
	logger *slog.Logger,
	renderer templates.Renderer,
	staticHandler http.Handler,
	apiClient *pilatescomplete.APIClient,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
	authenticationService *authentication.Service,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
	calendarsService *calendars.Service,
	statisticsService *statistics.Service,
) http.HandlerFunc {
	requireAuth := WithAuthentication(logger, authenticationService, credentialsStore)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", requireAuth(handleListEvents(logger, renderer, eventsService)))
	mux.HandleFunc("GET /statistics/year/{year}/{$}", requireAuth(handleYearStatistics(logger, renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/year/{year}/month/{month}/{$}", requireAuth(handleYearMonthStatistics(logger, renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/year/{year}/week/{week}/{$}", requireAuth(handleYearWeekStatistics(logger, renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/{$}", requireAuth(handleStatistics()))
	mux.HandleFunc("POST /{$}", handleLogin(logger, apiClient, credentialsStore, tokensStore))

	mux.HandleFunc("GET /login", handleAuthenticationPage(logger, renderer))

	mux.HandleFunc("POST /events/{event_id}/bookings", requireAuth(handleCreateBooking(logger, renderer, apiClient, eventsService, scheduler)))
	mux.HandleFunc("DELETE /events/{event_id}/bookings/{booking_id}", requireAuth(handleDeleteBooking(logger, renderer, eventsService, apiClient)))

	mux.HandleFunc("DELETE /events/{event_id}/jobs/{job_id}", requireAuth(handleDeleteJob(logger, renderer, eventsService, scheduler)))

	mux.HandleFunc("GET /calendars/{calendar_id}/pilatescomplete.ics", handleGetCalendar(logger, calendarsService))
	mux.HandleFunc("POST /calendars", requireAuth(handleCreateCalendar(logger, calendarsService)))

	mux.HandleFunc("GET /", staticHandler.ServeHTTP)

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

		event, err := bookOrScheduleEventBooking(r.Context(), eventID, eventsService, scheduler, apiClient)
		if err != nil {
			logger.Error("book or schedule event booking", "error", err)
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

func bookOrScheduleEventBooking(
	ctx context.Context,
	eventID string,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
	apiClient *pilatescomplete.APIClient,
) (*events.Event, error) {
	if _, err := apiClient.BookActivity(ctx, eventID); err != nil {
		if errors.Is(err, pilatescomplete.ErrActivityBookingTooEarly) {
			event, err := scheduleEventBooking(ctx, eventID, eventsService, scheduler)
			if err != nil {
				return nil, fmt.Errorf("schedule event booking: %w", err)
			}
			return event, nil
		}
		return nil, fmt.Errorf("book activity: %w", err)
	}

	event, err := eventsService.GetEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	return event, nil
}

func scheduleEventBooking(
	ctx context.Context,
	eventID string,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
) (*events.Event, error) {
	event, err := eventsService.GetEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	job, err := jobs.NewBookEventJob(ctx, eventID, event.BookableFrom)
	if err != nil {
		return nil, fmt.Errorf("new book event job: %w", err)
	}
	if err := scheduler.Schedule(ctx, job); err != nil {
		return nil, fmt.Errorf("schedule: %w", err)
	}

	event.Booking = &bookings.Booking{
		ID:     job.ID,
		Status: bookings.BookingStatusJobScheduled,
	}

	return event, nil
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

func handleStatistics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("/statistics/year/%d/", time.Now().Year()), http.StatusTemporaryRedirect)
	}
}

func handleYearWeekStatistics(
	logger *slog.Logger,
	renderer templates.Renderer,
	statisticsService *statistics.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		year, err := strconv.Atoi(parts[3])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		week, err := strconv.Atoi(parts[5])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		stats, err := statisticsService.CalculateYearWeek(r.Context(), year, week)
		if err != nil {
			logger.Error("calculate statistics by week", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		nextYear, nextWeek := getNextISOWeek(year, week)
		prevYear, prevWeek := getPreviousISOWeek(year, week)

		if err := renderer.RenderWeekStatisticsPage(w, templates.WeekStatisticsData{
			Total:    stats.Total,
			Year:     year,
			Month:    int(getMonthFromISOWeek(year, week)),
			Week:     week,
			PrevYear: prevYear,
			PrevWeek: prevWeek,
			NextYear: nextYear,
			NextWeek: nextWeek,
			Days:     stats.Days,
			Classes:  stats.Classes,
		}); err != nil {
			logger.Error("render month statistics page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleYearMonthStatistics(
	logger *slog.Logger,
	renderer templates.Renderer,
	statisticsService *statistics.Service,
) http.HandlerFunc {
	firstNonEmptyWeek := func(weeks []statistics.Week) int {
		for _, week := range weeks {
			if week.Total > 0 {
				return week.Number
			}
		}
		return weeks[len(weeks)-1].Number
	}
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		year, err := strconv.Atoi(parts[3])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		month, err := parseMonth(parts[5])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		stats, err := statisticsService.CalculateYearMonth(r.Context(), year, month)
		if err != nil {
			logger.Error("calculate statistics by year", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		nextYear, nextMonth := getNextMonth(year, month)
		prevYear, prevMonth := getPreviousMonth(year, month)
		if err := renderer.RenderMonthStatisticsPage(w, templates.MonthStatisticsData{
			Total:     stats.Total,
			Year:      year,
			Month:     int(month),
			Week:      firstNonEmptyWeek(stats.Weeks),
			PrevYear:  prevYear,
			PrevMonth: int(prevMonth),
			NextYear:  nextYear,
			NextMonth: int(nextMonth),
			Weeks:     stats.Weeks,
			Classes:   stats.Classes,
		}); err != nil {
			logger.Error("render month statistics page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleYearStatistics(
	logger *slog.Logger,
	renderer templates.Renderer,
	statisticsService *statistics.Service,
) http.HandlerFunc {
	firstNonEmptyMonth := func(months []statistics.Month) int {
		for _, month := range months {
			if month.Total > 0 {
				return month.Number
			}
		}
		return months[len(months)-1].Number
	}
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		year, err := strconv.Atoi(parts[3])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		stats, err := statisticsService.CalculateYear(r.Context(), year)
		if err != nil {
			logger.Error("calculate statistics by year", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := renderer.RenderYearStatisticsPage(w, templates.YearStatisticsData{
			Total:   stats.Total,
			Year:    year,
			Month:   firstNonEmptyMonth(stats.Months),
			Week:    1,
			Months:  stats.Months,
			Classes: stats.Classes,
		}); err != nil {
			logger.Error("render year statistics page", "error", err)
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
			logger.Error("list events", "error", err)
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

func getPreviousMonth(year int, month time.Month) (int, time.Month) {
	if month == time.January {
		return year - 1, time.December
	} else {
		return year, month - 1
	}
}

func getNextMonth(year int, month time.Month) (int, time.Month) {
	if month == time.December {
		return year + 1, time.January
	} else {
		return year, month + 1
	}
}

func getNextISOWeek(year int, week int) (nextYear int, nextWeek int) {
	// Create a time.Time for Monday of the given ISO week
	// Jan 4th is always in week 1 of its ISO year
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	// Get the Monday of week 1
	_, w1 := jan4.ISOWeek()
	daysToAdd := (week - w1) * 7
	targetDate := jan4.AddDate(0, 0, daysToAdd)

	// Add 7 days to get to next week
	nextDate := targetDate.AddDate(0, 0, 7)

	// Get the ISO week number for the next week
	nextYear, nextWeek = nextDate.ISOWeek()

	return nextYear, nextWeek
}

func getPreviousISOWeek(year int, week int) (prevYear int, prevWeek int) {
	// Create a time.Time for Monday of the given ISO week
	// Jan 4th is always in week 1 of its ISO year
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	// Get the Monday of week 1
	_, w1 := jan4.ISOWeek()
	daysToAdd := (week - w1) * 7
	targetDate := jan4.AddDate(0, 0, daysToAdd)

	// Subtract 7 days to get to previous week
	prevDate := targetDate.AddDate(0, 0, -7)

	// Get the ISO week number for the previous week
	prevYear, prevWeek = prevDate.ISOWeek()

	return prevYear, prevWeek
}

func getMonthFromISOWeek(year int, week int) time.Month {
	// Create a time.Time for Monday of the given ISO week
	// Jan 4th is always in week 1 of its ISO year
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	// Get the Monday of week 1
	_, w1 := jan4.ISOWeek()
	daysToAdd := (week - w1) * 7

	// Get the date of Monday in the requested week
	targetDate := jan4.AddDate(0, 0, daysToAdd)

	// Return the month
	return targetDate.Month()
}

func parseMonth(value string) (time.Month, error) {
	month, err := strconv.Atoi(value)
	if err != nil {
		return -1, err
	}
	if month < int(time.January) || month > int(time.December) {
		return -1, fmt.Errorf("invalid month number: %d", month)
	}
	return time.Month(month), nil
}
