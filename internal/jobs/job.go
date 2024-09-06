package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type Status uint

const (
	StatusUndefined = iota
	StatusPending
	StatusRunning
	StatusSucceded
	StatusFailing
)

type Job struct {
	ID       string      `json:"id"`
	Time     time.Time   `json:"time"`
	Status   Status      `json:"status"`
	Attempts []time.Time `json:"attempts"`
	Errors   []string    `json:"errors"`

	BookEvent *BookEventJob `json:"book_event,omitempty"`
}

type BookEventJob struct {
	EventID       string `json:"events_id"`
	CredentialsID string `json:"credentials_id"`
}

func (j Job) Do(ctx context.Context, s *Scheduler) error {
	if j.BookEvent != nil {
		_, err := s.authenticationService.AuthenticateContext(ctx, j.BookEvent.CredentialsID)
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
	eventID string,
	ts time.Time,
) (*Job, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("token missing from context")
	}
	return &Job{
		ID:     gonanoid.Must(),
		Status: StatusPending,
		Time:   ts,
		BookEvent: &BookEventJob{
			EventID:       eventID,
			CredentialsID: token.CredentialsID,
		},
	}, nil
}
