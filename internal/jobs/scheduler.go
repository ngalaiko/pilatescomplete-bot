package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type Scheduler struct {
	store                 *Store
	apiClient             *pilatescomplete.APIClient
	authenticationService *authentication.Service

	timersGuard sync.RWMutex
	timers      map[string]*time.Timer
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
		timers:                map[string]*time.Timer{},
	}
}

// Init will load all pending jobs from database into memeory, and start watching them.
func (s *Scheduler) Init(ctx context.Context) error {
	jobs, err := s.store.ListJobs(ctx, ByStatus(JobStatusPending, JobStatusFailing, JobStatusRunning))
	if err != nil {
		return err
	}
	for _, job := range jobs {
		s.setupTimerForJob(job)
	}
	return nil
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
	s.deleteTimer(job)
	log.Printf("[INFO] deleted job %q", job.ID)
	return nil
}

func (s *Scheduler) Schedule(ctx context.Context, job *Job) error {
	if err := s.store.InsertJob(ctx, job); err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	s.setupTimerForJob(job)
	return nil
}

func (s *Scheduler) deleteTimer(job *Job) {
	s.timersGuard.Lock()
	delete(s.timers, job.ID)
	s.timersGuard.Unlock()
}

func (s *Scheduler) setupTimerForJob(job *Job) {
	duration := time.Until(job.Time)
	log.Printf("[INFO] scheduled job %q to run at %s", job.ID, job.Time)
	s.timersGuard.Lock()
	s.timers[job.ID] = time.AfterFunc(duration, func() {
		s.runJob(job)
	})
	s.timersGuard.Unlock()
}

func (s *Scheduler) runJob(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("[INFO] starting job %q", job.ID)

	job.Status = JobStatusRunning
	job.Attempts = append(job.Attempts, time.Now())

	if err := s.store.InsertJob(ctx, job); err != nil {
		log.Printf("[ERROR] failed update job %q: %s", job.ID, err)
		return
	}

	if err := job.Do(ctx, s); err != nil {
		log.Printf("[ERROR] job %q failed: %s", job.ID, err)
		job.Errors = append(job.Errors, err.Error())
		job.Status = JobStatusFailing
		job.Status = JobStatusSucceded
		if next := nextRetry(job); next != nil {
			job.Time = *next
			s.setupTimerForJob(job)
		}
	} else {
		log.Printf("[INFO] job %q succeeded", job.ID)
		job.Errors = append(job.Errors, "")
	}

	if err := s.store.InsertJob(ctx, job); err != nil {
		log.Printf("[ERROR] failed update job %q: %s", job.ID, err)
		return
	}
}

func nextRetry(job *Job) *time.Time {
	if len(job.Attempts) == 5 {
		return nil
	}
	next := job.Time.Add(100 * time.Millisecond * 2 << len(job.Attempts))
	return &next
}
