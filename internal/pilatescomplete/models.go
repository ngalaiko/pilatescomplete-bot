package pilatescomplete

type Event struct {
	ActivityLocation Location     `json:"ActivityLocation"`
	ActivityType     ActivityType `json:"ActivityType"`
	Activity         Activity     `json:"Activity"`
	User             User         `json:"User"`
	Unbookable       bool         `json:"unbookable"`
	Booked           bool         `json:"booked"`
	Bookable         bool         `json:"bookable"`
	Reservable       bool         `json:"reservable"`
	ExtraLinks       []any        `json:"extra_links"`
	Past             bool         `json:"past"`
	Links            []any        `json:"links"`
}

type Activity struct {
	ID                   string `json:"id"`
	ActivityLocationID   string `json:"activity_location_id"`
	ActivityTypeID       string `json:"activity_type_id"`
	UserID               string `json:"user_id"`
	Start                string `json:"start"`
	Length               string `json:"length"`
	DisableCost          bool   `json:"disable_cost"`
	Bookable             bool   `json:"bookable"`
	TryIt                bool   `json:"try_it"`
	Places               string `json:"places"`
	PlacesTryit          string `json:"places_tryit"`
	Reserves             string `json:"reserves"`
	BookingPlacesCount   string `json:"booking_places_count"`
	BookingReservesCount string `json:"booking_reserves_count"`
	BookingCheckedCount  string `json:"booking_checked_count"`
	BookingMissedCount   string `json:"booking_missed_count"`
	BookingTryitCount    string `json:"booking_tryit_count"`
	Notice               string `json:"notice"`
	Canceled             any    `json:"canceled"`
	CancelReason         any    `json:"cancel_reason"`
	Modified             string `json:"modified"`
	RUsers               string `json:"r_users"`
	PlacesLeft           string `json:"places_left"`
	ReservesLeft         string `json:"reserves_left"`
	PlacesFull           string `json:"places_full"`
	ReservesFull         string `json:"reserves_full"`
}

type Location struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ActivityType struct {
	Name                      string `json:"name"`
	Color                     string `json:"color"`
	ColorNew                  string `json:"color_new"`
	ColorFixed                bool   `json:"color_fixed"`
	ColorShade                any    `json:"color_shade"`
	BookableTimes             bool   `json:"bookable_times"`
	BookableTimeEvery         string `json:"bookable_time_every"`
	BookableLengths           string `json:"bookable_lengths"`
	BookableSimultaneously    bool   `json:"bookable_simultaneously"`
	BookableBlockUserTimes    bool   `json:"bookable_block_user_times"`
	UseBookableTimeEveryStart bool   `json:"use_bookable_time_every_start"`
	BookingTryItText          string `json:"booking_try_it_text"`
	LateBookMinutes           string `json:"late_book_minutes"`
	LateUnbookMinutes         string `json:"late_unbook_minutes"`
	DaysInFutureBook          string `json:"days_in_future_book"`
	MultipleParticipants      bool   `json:"multiple_participants"`
	UsingResources            bool   `json:"using_resources"`
	BookingManyTryIt          bool   `json:"booking_many_try_it"`
	Modified                  string `json:"modified"`
	BookingsVisiblePublic     bool   `json:"bookings_visible_public"`
	QuestionActive            bool   `json:"question_active"`
	QuestionMandatory         bool   `json:"question_mandatory"`
	QuestionLabel             string `json:"question_label"`
	UseParentTypeResource     bool   `json:"use_parent_type_resource"`
	ID                        string `json:"id"`
	HasSubTypes               string `json:"has_sub_types"`
}

type User struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ID        string `json:"id"`
	ImageSrc  string `json:"image_src"`
}
