package events

import (
	"fmt"
	"time"

	"github.com/pilatescompletebot/internal/bookings"
	"github.com/pilatescompletebot/internal/pilatescomplete"
)

type ID string

type EventAvailability uint

const (
	EventAvailabilityUnknown = iota
	EventAvailabilityBookable
	EventAvailabilityReservable
	EventAvailabilityUnavailable
)

type Event struct {
	ID       ID
	Location string
	Name     string
	Time     time.Time
	Booking  *bookings.Booking

	PlacesTotal   int64
	PlacesTaken   int64
	ReservesTotal int64
	ReservesTaken int64
}

func (e Event) Bookable() bool {
	return e.PlacesTaken < e.PlacesTotal
}

func (e Event) Reservable() bool {
	return e.ReservesTaken < e.ReservesTotal
}

func EventFromAPI(event pilatescomplete.Event) (*Event, error) {
	var booking *bookings.Booking
	if event.ActivityBooking != nil {
		var err error
		booking, err = bookings.FromAPI(*event.ActivityBooking)
		if err != nil {
			return nil, err
		}
	}
	return &Event{
		ID:            ID(event.Activity.ID),
		Location:      event.ActivityLocation.Name,
		Name:          event.ActivityType.Name,
		Time:          event.Activity.Start.Time(),
		Booking:       booking,
		PlacesTotal:   event.Activity.Places.Int64(),
		PlacesTaken:   event.Activity.BookingPlacesCount.Int64(),
		ReservesTotal: event.Activity.Reserves.Int64(),
		ReservesTaken: event.Activity.BookingReservesCount.Int64(),
	}, nil
}

func EventsFromAPI(events []pilatescomplete.Event) ([]*Event, error) {
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
