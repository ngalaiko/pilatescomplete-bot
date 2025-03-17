package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db *badger.DB
}

func NewStore(db *badger.DB) *Store {
	return &Store{
		db: db,
	}
}

func (s *Store) InsertJob(_ context.Context, job *Job) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		if err := txn.Set(idKey(job.ID), data); err != nil {
			return err
		}
		return nil
	})
}

var ErrNotFound = errors.New("not found")

func ExcludeFailed() func(*Job) bool {
	return func(job *Job) bool {
		return len(job.Attempts) < MAX_ATTEMPTS
	}
}

func ExcludeSuccseeded() func(*Job) bool {
	return func(job *Job) bool {
		return job.Status != StatusSucceded
	}
}

func BookEventsByCredentialsIDEventIDs(credentialsID string, eventIDs ...string) func(*Job) bool {
	eventIDsfilter := make(map[string]bool, len(eventIDs))
	for _, s := range eventIDs {
		eventIDsfilter[s] = true
	}
	return func(job *Job) bool {
		if job.BookEvent == nil {
			return false
		}
		if job.BookEvent.CredentialsID != credentialsID {
			return false
		}
		return eventIDsfilter[job.BookEvent.EventID]
	}
}

func ByStatus(status ...Status) func(*Job) bool {
	filter := make(map[Status]bool, len(status))
	for _, s := range status {
		filter[s] = true
	}
	return func(job *Job) bool {
		return filter[job.Status]
	}
}

func (s *Store) FindByID(ctx context.Context, id string) (*Job, error) {
	var job Job
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(idKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			return json.Unmarshal(value, &job)
		})
	}); err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (s *Store) ListJobs(_ context.Context, filters ...func(*Job) bool) ([]*Job, error) {
	var jobs []*Job
	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("jobs/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			// only consider id keys
			if len(bytes.Split(item.Key(), []byte("/"))) == 3 {
				continue
			}
			if err := item.Value(func(value []byte) error {
				job := &Job{}
				if err := json.Unmarshal(value, &job); err != nil {
					return err
				}
				for _, filter := range filters {
					if !filter(job) {
						return nil
					}
				}
				jobs = append(jobs, job)
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

func (s *Store) DeleteJob(_ context.Context, id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(idKey(id))
	})
}

func idKey(id string) []byte {
	return []byte(fmt.Sprintf("jobs/%s", id))
}
