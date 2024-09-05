package events

import (
	"context"
	"fmt"
	"time"

	"github.com/pilatescomplete-bot/internal/bookings"
	"github.com/pilatescomplete-bot/internal/jobs"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type Service struct {
	apiClient *pilatescomplete.APIClient
	jobsStore *jobs.Store
}

func NewService(
	jobsStore *jobs.Store,
	apiClient *pilatescomplete.APIClient,
) *Service {
	return &Service{
		jobsStore: jobsStore,
		apiClient: apiClient,
	}
}

type ListEventsInput struct {
	From *time.Time
	To   *time.Time
}

func (s *Service) ListEvents(ctx context.Context, input ListEventsInput) ([]*Event, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("token missing from context")
	}
	apiResponse, err := s.apiClient.ListEvents(ctx, pilatescomplete.ListEventsInput{
		From: input.From,
		To:   input.To,
	})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	events, err := EventsFromAPI(apiResponse.Events)
	if err != nil {
		return nil, fmt.Errorf("events from api: %w", err)
	}
	eventIDs := make([]string, 0, len(events))
	eventsByID := make(map[string]*Event, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
		eventsByID[event.ID] = event
	}
	bookingJobs, err := s.jobsStore.ListJobs(ctx, jobs.BookEventsByCredentialsIDEventIDs(token.CredentialsID, eventIDs...))
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	for _, job := range bookingJobs {
		eventsByID[job.BookEvent.EventID].Booking = &bookings.Booking{
			ID:     job.ID,
			Status: bookings.BookingStatusJobScheduled,
		}
	}
	return events, nil
}
