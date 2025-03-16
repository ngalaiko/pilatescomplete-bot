package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescomplete-bot/internal/keys"
)

type Store struct {
	db            *badger.DB
	encryptionKey *keys.Key
}

func NewStore(
	db *badger.DB,
) *Store {
	return &Store{
		db: db,
	}
}

func (s *Store) InsertChat(ctx context.Context, chat *Chat) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(chat)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(fmt.Sprintf("telegram/chats/%d", chat.ID)), data); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) ListChats(ctx context.Context) ([]Chat, error) {
	chats := make([]Chat, 0)
	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("telegram/chats/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var chat Chat
			if err := item.Value(func(value []byte) error {
				return json.Unmarshal(value, &chat)
			}); err != nil {
				return err
			}
			chats = append(chats, chat)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return chats, nil
}

func (s *Store) SetUpdatesOffset(ctx context.Context, offset int) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data := []byte(fmt.Sprintf("%d", offset))
		return txn.Set([]byte("telegram/updates/offset"), data)
	})
}

func (s *Store) GetUpdatesOffset(ctx context.Context) (int, error) {
	var offset int
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("telegram/updates/offset"))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			_, err := fmt.Sscanf(string(value), "%d", &offset)
			return err
		})
	}); err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return offset, nil
}
