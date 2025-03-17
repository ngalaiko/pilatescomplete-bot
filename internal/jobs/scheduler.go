package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type Scheduler struct {
	store                 *Store
	apiClient             *pilatescomplete.APIClient
	authenticationService *authentication.Service

	jobsGuard sync.RWMutex
	jobs      map[string]*Job

	jobFailedCallbacks    []func(context.Context, *Job)
	jobSucceededCallbacks []func(context.Context, *Job)
}

func NewScheduler(
	store *Store,
	apiClient *pilatescomplete.APIClient,
	authenticationService *authentication.Service,
) *Scheduler {
	return &Scheduler{
		store:                 store,
		apiClient:             apiClient,
		authenticationService: authenticationService,
		jobs:                  make(map[string]*Job),
	}
}

func (s *Scheduler) OnJobFailed(cb func(context.Context, *Job)) {
	s.jobFailedCallbacks = append(s.jobFailedCallbacks, cb)
}

func (s *Scheduler) OnJobSucceeded(cb func(context.Context, *Job)) {
	s.jobSucceededCallbacks = append(s.jobSucceededCallbacks, cb)
}

// Init will load all pending jobs from database into memeory, and start watching them.
func (s *Scheduler) Init(ctx context.Context) error {
	jobs, err := s.store.ListJobs(ctx, ByStatus(StatusPending, StatusFailing, StatusRunning))
	if err != nil {
		return err
	}
	for _, job := range jobs {
		s.setupTimerForJob(ctx, job)
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobsToRun := []*Job{}
				s.jobsGuard.RLock()
				for _, job := range s.jobs {
					if time.Now().After(job.Time) {
						jobsToRun = append(jobsToRun, job)
					}
				}
				s.jobsGuard.RUnlock()

				for _, job := range jobsToRun {
					slog.InfoContext(ctx, "starting job", "job_id", job.ID, "attempt", len(job.Attempts))
					if err := s.runJob(ctx, job); err != nil {
						for _, cb := range s.jobFailedCallbacks {
							cb(ctx, job)
						}
					} else {
						for _, cb := range s.jobSucceededCallbacks {
							cb(ctx, job)
						}
					}
				}
			}
		}
	}()

	return nil
}

func (s *Scheduler) FindByID(ctx context.Context, id string) (*Job, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("token missing from context")
	}
	job, err := s.store.FindByID(ctx, id)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if job.BookEvent.CredentialsID != token.CredentialsID {
		return nil, ErrNotFound
	}
	return job, nil
}

func (s *Scheduler) DeleteByID(ctx context.Context, id string) error {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return fmt.Errorf("token missing from context")
	}
	job, err := s.store.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find by id: %w", err)
	}
	if job.BookEvent.CredentialsID != token.CredentialsID {
		return ErrNotFound
	}
	if err := s.store.DeleteJob(ctx, job.ID); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	s.deleteTimer(ctx, job)
	slog.InfoContext(ctx, "deleted job", "job_id", job.ID)
	return nil
}

func (s *Scheduler) Schedule(ctx context.Context, job *Job) error {
	if err := s.store.InsertJob(ctx, job); err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	s.setupTimerForJob(ctx, job)
	return nil
}

func (s *Scheduler) deleteTimer(ctx context.Context, job *Job) {
	s.jobsGuard.Lock()
	delete(s.jobs, job.ID)
	s.jobsGuard.Unlock()
	slog.InfoContext(ctx, "unscheduled job", "job_id", job.ID)
}

func (s *Scheduler) setupTimerForJob(ctx context.Context, job *Job) {
	s.jobsGuard.Lock()
	s.jobs[job.ID] = job
	s.jobsGuard.Unlock()
	slog.InfoContext(ctx, "scheduled job", "job_id", job.ID)
}

func (s *Scheduler) runJob(ctx context.Context, job *Job) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	job.Status = StatusRunning
	job.Attempts = append(job.Attempts, time.Now())

	if err := s.store.InsertJob(ctx, job); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	jobError := job.Do(ctx, s)
	if jobError != nil {
		job.Errors = append(job.Errors, jobError.Error())
		job.Status = StatusFailing
		if next := nextRetry(job); next != nil {
			job.Time = *next
		} else {
			s.deleteTimer(ctx, job)
		}
	} else {
		job.Status = StatusSucceded
		job.Errors = append(job.Errors, "")
		s.deleteTimer(ctx, job)
	}

	if err := s.store.InsertJob(ctx, job); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	return jobError
}

func nextRetry(job *Job) *time.Time {
	if len(job.Attempts) == 5 {
		return nil
	}
	next := job.Time.Add(100 * time.Millisecond * 2 << len(job.Attempts))
	return &next
}
