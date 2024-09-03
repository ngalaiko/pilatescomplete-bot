package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
)

type Scheduler struct {
	db               *badger.DB
	apiClient        *pilatescomplete.APIClient
	tokensStore      *tokens.Store
	credentialsStore *credentials.Store

	timersGuard sync.RWMutex
	timers      map[ID]*time.Timer
}

func NewScheduler(
	db *badger.DB,
	apiClient *pilatescomplete.APIClient,
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
) *Scheduler {
	return &Scheduler{
		db:               db,
		apiClient:        apiClient,
		tokensStore:      tokensStore,
		credentialsStore: credentialsStore,
		timers:           map[ID]*time.Timer{},
	}
}

// Init will load all pending jobs from database into memeory, and start watching them.
func (s *Scheduler) Init(ctx context.Context) error {
	jobs, err := s.listJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		s.setupTimerForJob(job)
	}
	return nil
}

func (s *Scheduler) Schedule(ctx context.Context, job *Job) error {
	if err := s.insertJob(ctx, job); err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	s.setupTimerForJob(job)
	return nil
}

func (s *Scheduler) setupTimerForJob(job *Job) {
	duration := time.Until(job.Time)
	log.Printf("[INFO] scheduled job %q to run after %s", job.ID, duration)
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

	if err := s.insertJob(ctx, job); err != nil {
		log.Printf("[ERROR] failed update job %q: %s", job.ID, err)
		return
	}

	if err := job.Do(ctx, s); err != nil {
		log.Printf("[ERROR] job %q failed: %s", job.ID, err)
		job.Errors = append(job.Errors, err.Error())
		job.Status = JobStatusFailing
		// TODO: retry job after some time
	} else {
		log.Printf("[ERROR] job %q succeededs", job.ID)
		job.Errors = append(job.Errors, "")
		job.Status = JobStatusSucceded
	}

	if err := s.insertJob(ctx, job); err != nil {
		log.Printf("[ERROR] failed update job %q: %s", job.ID, err)
		return
	}
}

func (s *Scheduler) insertJob(_ context.Context, job *Job) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		if err := txn.Set(idKey(job.ID), data); err != nil {
			return err
		}

		if job.BookEvent != nil {
			if err := txn.Set(bookEventKey(job.BookEvent), data); err != nil {
				return err
			}
		}
		return nil
	})
}

var ErrNotFound = errors.New("not found")

func (s *Scheduler) FindBookEventJob(
	ctx context.Context,
	bookEvent BookEventJob,
) (*Job, error) {
	job := &Job{}
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(bookEventKey(&bookEvent))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			return json.Unmarshal(value, &bookEvent)
		})
	}); err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return job, nil
}

func (s *Scheduler) listJobs(_ context.Context) ([]*Job, error) {
	var jobs []*Job
	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("jobs/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if err := item.Value(func(value []byte) error {
				job := &Job{}
				if err := json.Unmarshal(value, &job); err != nil {
					return err
				}
				if job.Status != JobStatusSucceded {
					jobs = append(jobs, job)
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return jobs, nil
}

func bookEventKey(bookEvent *BookEventJob) []byte {
	return []byte(fmt.Sprintf("jobs/%s/%s", bookEvent.CredentialsID, bookEvent.EventID))
}

func idKey(id ID) []byte {
	return []byte(fmt.Sprintf("jobs/%s", id))
}
