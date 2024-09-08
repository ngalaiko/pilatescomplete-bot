package calendars

import (
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

func (s *Store) InsertCalendar(_ context.Context, calendar *Calendar) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(calendar)
		if err != nil {
			return err
		}
		if err := txn.Set(idKey(calendar.ID), data); err != nil {
			return err
		}
		return nil
	})
}

var ErrNotFound = errors.New("not found")

func (s *Store) FindByID(ctx context.Context, id string) (*Calendar, error) {
	var calendar Calendar
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(idKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			return json.Unmarshal(value, &calendar)
		})
	}); err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &calendar, nil
}

func idKey(id string) []byte {
	return []byte(fmt.Sprintf("calendars/%s", id))
}
