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
	requireAuth := WithAuthentication(authenticationService, credentialsStore)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", requireAuth(handleListEvents(renderer, eventsService)))
	mux.HandleFunc("GET /statistics/year/{year}/{$}", requireAuth(handleYearStatistics(renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/year/{year}/month/{month}/{$}", requireAuth(handleYearMonthStatistics(renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/year/{year}/week/{week}/{$}", requireAuth(handleYearWeekStatistics(renderer, statisticsService)))
	mux.HandleFunc("GET /statistics/{$}", requireAuth(handleStatistics()))
	mux.HandleFunc("POST /{$}", handleLogin(apiClient, credentialsStore, tokensStore))

	mux.HandleFunc("GET /login", handleAuthenticationPage(renderer))

	mux.HandleFunc("POST /events/{event_id}/bookings", requireAuth(handleCreateBooking(renderer, apiClient, eventsService, scheduler)))
	mux.HandleFunc("DELETE /events/{event_id}/bookings/{booking_id}", requireAuth(handleDeleteBooking(renderer, eventsService, apiClient)))

	mux.HandleFunc("DELETE /events/{event_id}/jobs/{job_id}", requireAuth(handleDeleteJob(renderer, eventsService, scheduler)))

	mux.HandleFunc("GET /calendars/{calendar_id}/pilatescomplete.ics", handleGetCalendar(calendarsService))
	mux.HandleFunc("POST /calendars", requireAuth(handleCreateCalendar(calendarsService)))

	mux.HandleFunc("GET /", staticHandler.ServeHTTP)

	return WithAccessLogs()(mux.ServeHTTP)
}

func handleGetCalendar(calendarsService *calendars.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		id := parts[2]

		w.Header().Set("Content-Type", "text/calendar")
		if err := calendarsService.WriteICal(r.Context(), w, id); err != nil {
			slog.ErrorContext(r.Context(), "write calendar", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleCreateCalendar(
	calendarsService *calendars.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cal, err := calendarsService.CreateCalendar(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "create calendar", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		origin, err := url.Parse(r.Header.Get("Origin"))
		if err != nil {
			slog.ErrorContext(r.Context(), "parse origin", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("webcal://%s/calendars/%s/pilatescomplete.ics", origin.Host, cal.ID), http.StatusFound)
	}
}

func handleDeleteJob(
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
			slog.ErrorContext(r.Context(), "find job by id", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if job.BookEvent != nil && job.BookEvent.EventID != eventID {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := scheduler.DeleteByID(r.Context(), jobID); err != nil {
			slog.ErrorContext(r.Context(), "delete by id", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), job.BookEvent.EventID)
		if err != nil {
			slog.ErrorContext(r.Context(), "get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		if err := renderer.RenderEvent(w, event); err != nil {
			slog.ErrorContext(r.Context(), "render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleDeleteBooking(
	renderer templates.Renderer,
	eventsService *events.Service,
	apiClient *pilatescomplete.APIClient,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]
		bookingID := parts[4]

		if err := apiClient.CancelBooking(r.Context(), bookingID); err != nil {
			slog.ErrorContext(r.Context(), "cancal booking", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event, err := eventsService.GetEvent(r.Context(), eventID)
		if err != nil {
			slog.ErrorContext(r.Context(), "get event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		if err := renderer.RenderEvent(w, event); err != nil {
			slog.ErrorContext(r.Context(), "render event", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleCreateBooking(
	renderer templates.Renderer,
	apiClient *pilatescomplete.APIClient,
	eventsService *events.Service,
	scheduler *jobs.Scheduler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		eventID := parts[2]

		if err := r.ParseForm(); err != nil {
			slog.ErrorContext(r.Context(), "parse form", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		event, err := bookOrScheduleEventBooking(r.Context(), eventID, eventsService, scheduler, apiClient)
		if err != nil {
			slog.ErrorContext(r.Context(), "book or schedule event booking", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := renderer.RenderEvent(w, event); err != nil {
			slog.ErrorContext(r.Context(), "render event", "error", err)
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

func handleAuthenticationPage(renderer templates.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := renderer.RenderLoginPage(w, templates.LoginData{}); err != nil {
			slog.ErrorContext(r.Context(), "render login page", "error", err)
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
			slog.ErrorContext(r.Context(), "calculate statistics by week", "error", err)
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
			slog.ErrorContext(r.Context(), "render month statistics page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleYearMonthStatistics(
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
			slog.ErrorContext(r.Context(), "calculate statistics by year", "error", err)
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
			slog.ErrorContext(r.Context(), "render month statistics page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleYearStatistics(
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
			slog.ErrorContext(r.Context(), "calculate statistics by year", "error", err)
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
			slog.ErrorContext(r.Context(), "render year statistics page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func handleListEvents(
	renderer templates.Renderer,
	eventsService *events.Service,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := eventsService.ListEvents(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "list events", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := renderer.RenderEventsPage(w, templates.EventsData{
			Events: events,
		}); err != nil {
			slog.ErrorContext(r.Context(), "render events page", "error", err)
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
				slog.ErrorContext(r.Context(), "perse form", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			login, password := r.PostForm.Get("login"), r.PostForm.Get("password")

			cookie, err := client.Login(r.Context(), pilatescomplete.LoginData{
				Login:    login,
				Password: password,
			})
			if err != nil {
				slog.ErrorContext(r.Context(), "login", "error", err)
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
					slog.ErrorContext(r.Context(), "insert credentials", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			if err := tokensStore.Insert(r.Context(), &tokens.Token{
				CredentialsID: creds.ID,
				Token:         cookie.Value,
				Expires:       cookie.Expires,
			}); err != nil {
				slog.ErrorContext(r.Context(), "insert token", "error", err)
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
