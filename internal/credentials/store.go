package credentials

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

var ErrNotFound = errors.New("not found")

func NewStore(
	db *badger.DB,
	encryptionKey *keys.Key,
) *Store {
	return &Store{
		db:            db,
		encryptionKey: encryptionKey,
	}
}

func (s *Store) FindByID(ctx context.Context, id string) (*Credentials, error) {
	var credential EncodedCredentials
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(idKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(value []byte) error {
			return json.Unmarshal(value, &credential)
		})
	}); err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return credential.Decode(s.encryptionKey)
}

func (s *Store) FindByLogin(ctx context.Context, login string) (*Credentials, error) {
	var credential EncodedCredentials
	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(loginKey(login))
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
		if err := txn.Set(idKey(credential.ID), data); err != nil {
			return err
		}
		if err := txn.Set(loginKey(credential.Login), idKey(credential.ID)); err != nil {
			return err
		}
		return nil
	})
}

func idKey(id string) []byte {
	return []byte(fmt.Sprintf("credentials/%s", id))
}

func loginKey(login string) []byte {
	return []byte(fmt.Sprintf("logins/%s", login))
}
