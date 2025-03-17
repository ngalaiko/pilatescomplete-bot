package migrations

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/dgraph-io/badger/v4"
)

func Run(db *badger.DB) error {
	if err := renameCredentialsLoginsKey(db); err != nil {
		return fmt.Errorf("rename credentials logins key: %w", err)
	}
	return nil
}

func renameCredentialsLoginsKey(db *badger.DB) error {
	return db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("credentials/login")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			newKey := bytes.TrimPrefix(it.Item().Key(), []byte("credentials/"))
			if err := it.Item().Value(func(value []byte) error {
				return txn.Set(newKey, value)
			}); err != nil {
				return fmt.Errorf("failed to set new key: %w", err)
			}
			if err := txn.Delete(it.Item().Key()); err != nil {
				return fmt.Errorf("delete old key: %w", err)
			}
			slog.Info(
				"credentials migrated",
				"old_ley", string(it.Item().Key()),
				"new_key", string(newKey))
		}
		return nil
	})
}
