package events

import (
	"fmt"
	"time"

	"github.com/pilatescomplete-bot/internal/bookings"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
)

type EventAvailability uint

const (
	EventAvailabilityUnknown = iota
	EventAvailabilityBookable
	EventAvailabilityReservable
	EventAvailabilityUnavailable
)

type Event struct {
	ID string
	// LocationDisplayName is a name of the event's location
	LocationDisplayName string
	// DisplayName is a display name of the event
	DisplayName string
	// DisplayNotice is a display name subtitle of the event
	DisplayNotice string
	// StartTime is a time when event starts
	StartTime time.Time
	// StartTime is a time when event starts
	EndTime time.Time
	// BookableFrom is a time from when event can be booked / reserved
	BookableFrom time.Time
	// Booking contains an active booking for the event
	Booking *bookings.Booking
	Color   string

	PlacesTotal   int64
	PlacesTaken   int64
	ReservesTotal int64
	ReservesTaken int64
}

func (e Event) Duration() time.Duration {
	return e.EndTime.Sub(e.StartTime)
}

func (e Event) Bookable() bool {
	return e.PlacesTaken < e.PlacesTotal
}

func (e Event) Reservable() bool {
	return e.ReservesTaken < e.ReservesTotal
}

func (e Event) FullyBooked() bool {
	return !e.Reservable() && !e.Bookable()
}

var (
	minute = time.Second * 60
	hour   = minute * 60
	day    = hour * 24
)

// calculateBookableFrom returns timestame from wich an event can be booke.
// it's 07:00:01 on the day event occurs.
func calculateBookableFrom(event *pilatescomplete.Event) time.Time {
	ts := event.Activity.Start.Time().Add(-day * time.Duration(event.ActivityType.DaysInFutureBook.Int64()))
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 7, 0, 1, 0, ts.Location())
}

func EventFromAPI(event *pilatescomplete.Event) (*Event, error) {
	var booking *bookings.Booking
	if event.ActivityBooking != nil {
		var err error
		booking, err = bookings.FromAPI(*event.ActivityBooking)
		if err != nil {
			return nil, err
		}
	}
	return &Event{
		ID:                  event.Activity.ID,
		LocationDisplayName: event.ActivityLocation.Name,
		DisplayNotice:       event.Activity.Notice,
		DisplayName:         event.ActivityType.Name,
		StartTime:           event.Activity.Start.Time(),
		EndTime:             event.Activity.Start.Time().Add(minute * time.Duration(event.Activity.Length.Int64())),
		Booking:             booking,
		PlacesTotal:         event.Activity.Places.Int64(),
		PlacesTaken:         event.Activity.BookingPlacesCount.Int64(),
		ReservesTotal:       event.Activity.Reserves.Int64(),
		ReservesTaken:       event.Activity.BookingReservesCount.Int64(),
		BookableFrom:        calculateBookableFrom(event),
		Color:               event.ActivityType.ColorNew,
	}, nil
}

func EventsFromAPI(events []*pilatescomplete.Event) ([]*Event, error) {
	out := make([]*Event, len(events))
	for i := range events {
		var err error
		out[i], err = EventFromAPI(events[i])
		if err != nil {
			return nil, fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	return out, nil
}
