package credentials

import (
	"context"
	"encoding/json"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/keys"
)

type Store struct {
	db            *badger.DB
	encryptionKey *keys.Key
}

func NewStore(
	db *badger.DB,
	encryptionKey *keys.Key,
) *Store {
	return &Store{
		db:            db,
		encryptionKey: encryptionKey,
	}
}

func (s *Store) FindByID(ctx context.Context, id ID) (*Credentials, error) {
	var credential EncodedCredentials
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(id))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			return json.Unmarshal(value, &credential)
		})
	}); err != nil {
		return nil, err
	}
	return credential.Decode(s.encryptionKey)
}

func (s *Store) FindByLogin(ctx context.Context, login string) (*Credentials, error) {
	var credential EncodedCredentials
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(login))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			item, err := txn.Get(value)
			if err != nil {
				return err
			}
			return item.Value(func(value []byte) error {
				return json.Unmarshal(value, &credential)
			})
		})
	}); err != nil {
		return nil, err
	}
	return credential.Decode(s.encryptionKey)
}

func (s *Store) Insert(ctx context.Context, credential *Credentials) error {
	encoded, err := credential.Encode(s.encryptionKey)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(encoded)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(encoded.ID), data); err != nil {
			return err
		}
		if err := txn.Set([]byte(encoded.Login), []byte(encoded.ID)); err != nil {
			return err
		}
		return nil
	})
}
