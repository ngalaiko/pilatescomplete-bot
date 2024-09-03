package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/events"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type ID string

func NewID() ID {
	return ID(gonanoid.Must())
}

type JobStatus uint

const (
	JobStatusUndefined = iota
	JobStatusPending
	JobStatusRunning
	JobStatusSucceded
	JobStatusFailing
)

type Job struct {
	ID       ID          `json:"id"`
	Time     time.Time   `json:"time"`
	Status   JobStatus   `json:"status"`
	Attempts []time.Time `json:"attempts"`
	Errors   []string    `json:"errors"`

	BookEvent *BookEventJob `json:"book_event,omitempty"`
}

type BookEventJob struct {
	CredentialsID credentials.ID `json:"credentials_id"`
	EventID       events.ID      `json:"events_id"`
}

func (j Job) Do(ctx context.Context, s *Scheduler) error {
	if j.BookEvent != nil {
		ctx, err := s.authenticationService.AuthenticateContext(ctx, j.BookEvent.CredentialsID)
		if err != nil {
			return fmt.Errorf("authenticate context: %w", err)
		}

		if _, err := s.apiClient.BookActivity(ctx, string(j.BookEvent.EventID)); errors.Is(err, pilatescomplete.ErrActivityAlreadyBooked) {
			return nil
		} else if err != nil {
			return fmt.Errorf("book activity: %w", err)
		}

		return nil
	}
	return fmt.Errorf("unsupported job type")
}

func NewBookEventJob(
	ctx context.Context,
	eventID events.ID,
	ts time.Time,
) (*Job, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("token missing from context")
	}
	return &Job{
		ID:     NewID(),
		Status: JobStatusPending,
		Time:   ts,
		BookEvent: &BookEventJob{
			CredentialsID: token.CredentialsID,
			EventID:       eventID,
		},
	}, nil
}
