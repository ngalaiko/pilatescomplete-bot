package bookings

import (
	"fmt"

	"github.com/pilatescompletebot/internal/pilatescomplete"
)

type ID string

type BookingStatus uint

const (
	BookingStatusUnknown = iota
	BookingStatusReserved
	BookingStatusBooked
)

type Booking struct {
	ID     ID
	Status BookingStatus
	// Position conains position in a queue if status is Reserved
	Position int64
}

func (b Booking) IsBooked() bool {
	return b.Status == BookingStatusBooked
}

func (b Booking) IsReserved() bool {
	return b.Status == BookingStatusReserved
}

func FromAPI(booking pilatescomplete.ActivityBooking) (*Booking, error) {
	status, err := statusFromAPI(booking.Status)
	if err != nil {
		return nil, err
	}
	return &Booking{
		ID:       ID(booking.BookingID),
		Status:   status,
		Position: booking.Position.Int64(),
	}, nil
}

func statusFromAPI(status pilatescomplete.ActivityBookingStatus) (BookingStatus, error) {
	switch status {
	case pilatescomplete.ActivityBookingStatusBooked:
		return BookingStatusBooked, nil
	case pilatescomplete.ActivityBookingStatusReserved:
		return BookingStatusReserved, nil
	default:
		return BookingStatusUnknown, fmt.Errorf("%q: unknown booking status", status)
	}
}
