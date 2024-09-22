package events

import (
	"context"
	"errors"
	"fmt"

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

var ErrNotFound = errors.New("not found")

func (s *Service) GetEvent(ctx context.Context, id string) (*Event, error) {
	events, err := s.listEvents(ctx, pilatescomplete.ListEventsInput{
		ActivityID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrNotFound
	}
	return events[0], nil
}

func (s *Service) ListEvents(ctx context.Context) ([]*Event, error) {
	return s.listEvents(ctx, pilatescomplete.ListEventsInput{})
}

func (s *Service) listEvents(ctx context.Context, input pilatescomplete.ListEventsInput) ([]*Event, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("token missing from context")
	}
	apiResponse, err := s.apiClient.ListEvents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	events, err := eventsFromAPI(apiResponse)
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
		if job.Status == jobs.StatusSucceded {
			continue
		}
		eventsByID[job.BookEvent.EventID].Booking = &bookings.Booking{
			ID:     job.ID,
			Status: bookings.BookingStatusJobScheduled,
		}
	}
	return events, nil
}
