package events

import (
	"fmt"
	"strings"
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
	Booking     *bookings.Booking
	TrainerName string
	Description string

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

func eventsFromAPI(events *pilatescomplete.ListEventsResponse) ([]*Event, error) {
	out := make([]*Event, len(events.Events))
	for i := range events.Events {
		event := events.Events[i]
		var booking *bookings.Booking
		if event.ActivityBooking != nil {
			var err error
			booking, err = bookings.FromAPI(*event.ActivityBooking)
			if err != nil {
				return nil, fmt.Errorf("events[%d]: %w", i, err)
			}
		}
		out[i] = &Event{
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
			TrainerName:         userName(&event.User),
			Description:         events.ActicityTypeDescriptions[event.Activity.ActivityTypeID],
		}
	}
	return out, nil
}

func userName(user *pilatescomplete.User) string {
	nonEmpty := []string{}
	for _, part := range append(strings.Split(user.FirstName, " "), strings.Split(user.LastName, "")...) {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, " ")
}
