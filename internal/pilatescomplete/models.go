package pilatescomplete

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/pilatescomplete-bot/internal/timezone"
)

type ActivityBookingStatus string

const (
	ActivityBookingStatusBooked   = "ok"
	ActivityBookingStatusMissed   = "missed"
	ActivityBookingStatusReserved = "reserved"
	ActivityBookingStatusChecked  = "checked"
)

type ActivityBooking struct {
	BookingID string                `json:"id"`
	Status    ActivityBookingStatus `json:"status"`
	Position  Int64String           `json:"position"`
}

type NullableInt64String struct {
	Int64String
	valid bool
}

func (u NullableInt64String) Valid() bool {
	return u.valid
}

func (u *NullableInt64String) UnmarshalJSON(data []byte) error {
	if string(data) == "\"\"" {
		u.valid = false
		return nil
	}
	u.valid = true
	return u.Int64String.UnmarshalJSON(data)
}

func (u NullableInt64String) Int64() int64 {
	if !u.valid {
		return 0
	}
	return u.Int64String.Int64()
}

// Int64String is a srting that contains an integer number.
type Int64String int64

func (u Int64String) Int64() int64 {
	return int64(u)
}

func (u *Int64String) UnmarshalJSON(data []byte) error {
	data = bytes.TrimFunc(data, func(r rune) bool {
		return r == rune(34)
	})
	n, err := strconv.ParseInt(string(data), 10, 32)
	if err != nil {
		return err
	}
	*u = Int64String(n)
	return nil
}

// BoolInt64String is a string that is either "1" or "0"
type BoolInt64String bool

func (u *BoolInt64String) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "\"1\"":
		*u = BoolInt64String(true)
		return nil
	case "\"0\"":
		*u = BoolInt64String(false)
		return nil
	default:
		return fmt.Errorf(`%q "0" or "1"`, string(data))
	}
}

// DateTime is date and time in stockholm, marshalled as 2006-01-02 15:04:05
type DateTime time.Time

func (d DateTime) Time() time.Time {
	return time.Time(d)
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(fmt.Sprintf("\"%s\"", time.DateTime), string(data))
	if err != nil {
		return err
	}
	*d = DateTime(timezone.InStockholm(t))
	return nil
}

type Event struct {
	ActivityLocation  Location         `json:"ActivityLocation"`
	ActivityType      ActivityType     `json:"ActivityType"`
	Activity          Activity         `json:"Activity"`
	User              []User           `json:"User"`
	ActivityBooking   *ActivityBooking `json:"MyActivityBooking"`
	ActivityBookingID string           `json:"activity_booking_id"`
	Unbookable        bool             `json:"unbookable"`
	Booked            bool             `json:"booked"`
	Bookable          bool             `json:"bookable"`
	Reservable        bool             `json:"reservable"`
}

type Activity struct {
	ID                   string          `json:"id"`
	ActivityLocationID   string          `json:"activity_location_id"`
	ActivityTypeID       string          `json:"activity_type_id"`
	UserID               string          `json:"user_id"`
	Start                DateTime        `json:"start"`
	Length               Int64String     `json:"length"`
	DisableCost          bool            `json:"disable_cost"`
	Bookable             bool            `json:"bookable"`
	TryIt                bool            `json:"try_it"`
	Places               Int64String     `json:"places"`
	PlacesTryit          Int64String     `json:"places_tryit"`
	Reserves             Int64String     `json:"reserves"`
	BookingPlacesCount   Int64String     `json:"booking_places_count"`
	BookingReservesCount Int64String     `json:"booking_reserves_count"`
	BookingCheckedCount  Int64String     `json:"booking_checked_count"`
	BookingMissedCount   Int64String     `json:"booking_missed_count"`
	BookingTryitCount    Int64String     `json:"booking_tryit_count"`
	Notice               string          `json:"notice"`
	Canceled             any             `json:"canceled"`
	CancelReason         any             `json:"cancel_reason"`
	Modified             DateTime        `json:"modified"`
	PlacesLeft           Int64String     `json:"places_left"`
	ReservesLeft         Int64String     `json:"reserves_left"`
	PlacesFull           BoolInt64String `json:"places_full"`
	ReservesFull         BoolInt64String `json:"reserves_full"`
}

type Location struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ActivityType struct {
	ID                        string              `json:"id"`
	Name                      string              `json:"name"`
	Color                     string              `json:"color"`
	ColorNew                  string              `json:"color_new"`
	ColorFixed                bool                `json:"color_fixed"`
	ColorShade                any                 `json:"color_shade"`
	BookableTimes             bool                `json:"bookable_times"`
	BookableTimeEvery         string              `json:"bookable_time_every"`
	BookableLengths           string              `json:"bookable_lengths"`
	BookableSimultaneously    bool                `json:"bookable_simultaneously"`
	BookableBlockUserTimes    bool                `json:"bookable_block_user_times"`
	UseBookableTimeEveryStart bool                `json:"use_bookable_time_every_start"`
	BookingTryItText          string              `json:"booking_try_it_text"`
	LateBookMinutes           Int64String         `json:"late_book_minutes"`
	LateUnbookMinutes         Int64String         `json:"late_unbook_minutes"`
	DaysInFutureBook          NullableInt64String `json:"days_in_future_book"`
	MultipleParticipants      bool                `json:"multiple_participants"`
	UsingResources            bool                `json:"using_resources"`
	BookingManyTryIt          bool                `json:"booking_many_try_it"`
	Modified                  string              `json:"modified"`
	BookingsVisiblePublic     bool                `json:"bookings_visible_public"`
	QuestionActive            bool                `json:"question_active"`
	QuestionMandatory         bool                `json:"question_mandatory"`
	QuestionLabel             string              `json:"question_label"`
	UseParentTypeResource     bool                `json:"use_parent_type_resource"`
	HasSubTypes               string              `json:"has_sub_types"`
}

type User struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ID        string `json:"id"`
	ImageSrc  string `json:"image_src"`
}

type NotificationType string

const (
	NoticicationTypeUnbooked = "WBOOKING_CONFIRMATION_UNBOOK"
	NoticicationTypeBooked   = "WBOOKING_CONFIRMATION"
	NotificationTypeGotPlace = "WBOOKING_RESERVE_GOT_PLACE"
)

type Notification struct {
	ID           string           `json:"id"`
	Type         NotificationType `json:"type"`
	Notification string           `json:"notification"`
	Created      DateTime         `json:"created"`
}
