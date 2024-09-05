package timezone

import "time"

func mustLoadLocation(zone string) *time.Location {
	location, err := time.LoadLocation(zone)
	if err != nil {
		panic(err)
	}
	return location
}

var stockholmLocation = mustLoadLocation("Europe/Stockholm")

func InStockholm(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), stockholmLocation)
}
