package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/events"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
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
		token, err := s.tokensStore.FindByID(ctx, j.BookEvent.CredentialsID)
		if errors.Is(err, tokens.ErrNotFound) {
			creds, err := s.credentialsStore.FindByID(ctx, j.BookEvent.CredentialsID)
			if err != nil {
				return fmt.Errorf("find credentials: %w", err)
			}

			cookie, err := s.apiClient.Login(ctx, pilatescomplete.LoginData{
				Login:    creds.Login,
				Password: creds.Password,
			})
			if err != nil {
				return fmt.Errorf("login: %w", err)
			}

			token = &tokens.Token{
				CredentialsID: creds.ID,
				Token:         cookie.Value,
				Expires:       cookie.Expires,
			}

			if err := s.tokensStore.Insert(ctx, token); err != nil {
				return fmt.Errorf("insert token: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("find token: %w", err)
		}

		ctx := tokens.NewContext(ctx, token)

		if _, err := s.apiClient.BookActivity(ctx, string(j.BookEvent.EventID)); err != nil {
			// handle already booked
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
