package events

import (
	"fmt"
	"time"

	"github.com/pilatescompletebot/internal/pilatescomplete"
)

type ID string

type Event struct {
	ID       ID
	Location string
	Name     string
	Time     time.Time
}

func EventFromAPI(event *pilatescomplete.Event) (*Event, error) {
	start, err := time.Parse(time.DateTime, event.Activity.Start)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %w", err)
	}
	return &Event{
		ID:       ID(event.Activity.ID),
		Location: event.ActivityLocation.Name,
		Name:     event.ActivityType.Name,
		Time:     start,
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
