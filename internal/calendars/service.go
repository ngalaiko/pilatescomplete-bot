package calendars

import (
	"context"
	"fmt"
	"io"

	ics "github.com/arran4/golang-ical"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/bookings"
	"github.com/pilatescomplete-bot/internal/devices"
	"github.com/pilatescomplete-bot/internal/events"
)

type Service struct {
	store                 *Store
	authenticationService *authentication.Service
	eventsService         *events.Service
}

func NewService(
	store *Store,
	authenticationService *authentication.Service,
	eventsSerivce *events.Service,
) *Service {
	return &Service{
		store:                 store,
		authenticationService: authenticationService,
		eventsService:         eventsSerivce,
	}
}

func (s *Service) CreateCalendar(ctx context.Context) (*Calendar, error) {
	device, ok := devices.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("devices missing from context")
	}
	cal := &Calendar{
		ID:            gonanoid.Must(),
		CredentialsID: device.CredentialsID,
	}
	if err := s.store.InsertCalendar(ctx, cal); err != nil {
		return nil, fmt.Errorf("insert calendar: %w", err)
	}
	return cal, nil
}

func (s *Service) WriteICal(ctx context.Context, w io.Writer, id string) error {
	cal, err := s.store.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find by id %q: %w", id, err)
	}
	ctx, err = s.authenticationService.AuthenticateContext(ctx, cal.CredentialsID)
	if err != nil {
		return fmt.Errorf("authenticate context: %w", err)
	}
	events, err := s.eventsService.ListEvents(ctx)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	icalendar := ics.NewCalendar()
	icalendar.SetName("Pilates complete")
	for _, event := range events {
		if event.Booking == nil {
			continue
		}
		var displayName string
		switch event.Booking.Status {
		case bookings.BookingStatusBooked:
			displayName = fmt.Sprintf("[BOOKED] %s", event.DisplayName)
		case bookings.BookingStatusReserved, bookings.BookingStatusJobScheduled:
			displayName = fmt.Sprintf("[RESERVED] %s", event.DisplayName)
		default:
			continue
		}
		ievent := icalendar.AddEvent(event.Booking.ID)
		ievent.SetSummary(displayName)
		if event.DisplayNotice != "" {
			ievent.SetDescription(event.DisplayNotice)
		}
		ievent.SetLocation(event.LocationDisplayName)
		ievent.SetStartAt(event.StartTime)
		ievent.SetEndAt(event.EndTime)
	}
	return icalendar.SerializeTo(w)
}
