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

	jobsGuard sync.RWMutex
	jobs      map[string]*Job
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

// Init will load all pending jobs from database into memeory, and start watching them.
func (s *Scheduler) Init(ctx context.Context) error {
	jobs, err := s.store.ListJobs(ctx, ByStatus(StatusPending, StatusFailing, StatusRunning))
	if err != nil {
		return err
	}
	for _, job := range jobs {
		s.setupTimerForJob(job)
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
					s.runJob(job)
				}
			}
		}
	}()

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
	s.jobsGuard.Lock()
	delete(s.jobs, job.ID)
	s.jobsGuard.Unlock()
	log.Printf("[INFO] removed job %q  from scheduled", job.ID)
}

func (s *Scheduler) setupTimerForJob(job *Job) {
	s.jobsGuard.Lock()
	s.jobs[job.ID] = job
	s.jobsGuard.Unlock()
	log.Printf("[INFO] scheduled job %q to run at %s", job.ID, job.Time)
}

func (s *Scheduler) runJob(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("[INFO] starting job %q", job.ID)

	job.Status = StatusRunning
	job.Attempts = append(job.Attempts, time.Now())

	if err := s.store.InsertJob(ctx, job); err != nil {
		log.Printf("[ERROR] failed update job %q: %s", job.ID, err)
		return
	}

	if err := job.Do(ctx, s); err != nil {
		log.Printf("[ERROR] job %q failed: %s", job.ID, err)
		job.Errors = append(job.Errors, err.Error())
		job.Status = StatusFailing
		if next := nextRetry(job); next != nil {
			job.Time = *next
		}
	} else {
		log.Printf("[INFO] job %q succeeded", job.ID)
		job.Status = StatusSucceded
		job.Errors = append(job.Errors, "")
		s.deleteTimer(job)
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
