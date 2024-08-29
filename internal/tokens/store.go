package tokens

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
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

var ErrNotFound = errors.New("not found")

// FindByID returns first token for credentials id that did not expire.
func (s *Store) FindByID(ctx context.Context, credentialsID credentials.ID) (*Token, error) {
	var token EncodedToken
	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("id")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			keyParts := bytes.Split(key, []byte("/"))
			ts, err := strconv.ParseInt(string(keyParts[1]), 10, 64)
			if err != nil {
				panic(err)
			}
			t := time.Unix(ts, 0)
			if t.After(time.Now()) {
				return item.Value(func(value []byte) error {
					return json.Unmarshal(value, &token)
				})
			}
		}
		return ErrNotFound
	}); err != nil {
		return nil, err
	}
	return token.Decode(s.encryptionKey)
}

func (s *Store) Insert(ctx context.Context, token *Token) error {
	encoded, err := token.Encode(s.encryptionKey)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(encoded)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(fmt.Sprintf("%s/%d", encoded.CredentialsID, encoded.Expires.Unix())), data); err != nil {
			return err
		}
		return nil
	})
}
