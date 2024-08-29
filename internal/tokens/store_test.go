package tokens_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/keys"
	"github.com/pilatescompletebot/internal/tokens"
)

func TestFind(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key, err := keys.NewKey()
	if err != nil {
		t.Fatal(err)
	}

	store := tokens.NewStore(db, key)

	inserted := tokens.Token{
		CredentialsID: "id",
		Token:         "token",
		Expires:       time.Now().Add(100 * time.Second).Round(time.Millisecond),
	}

	insertedAndExpired := tokens.Token{
		CredentialsID: "id",
		Token:         "token",
		Expires:       time.Now().Add(-100 * time.Second).Round(time.Millisecond),
	}

	ctx := context.Background()
	if err := store.Insert(ctx, &inserted); err != nil {
		t.Fatal(err)
	}

	if err := store.Insert(ctx, &insertedAndExpired); err != nil {
		t.Fatal(err)
	}

	foundByID, err := store.FindByID(ctx, inserted.CredentialsID)
	if err != nil {
		t.Fatal(err)
	}

	if inserted != *foundByID {
		t.Logf("inserted: %+v", inserted)
		t.Logf("foundByID: %+v", *foundByID)
		t.Fatal("inserted != found")
	}
}

func TestFindExpired(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key, err := keys.NewKey()
	if err != nil {
		t.Fatal(err)
	}

	store := tokens.NewStore(db, key)

	inserted := tokens.Token{
		CredentialsID: "id",
		Token:         "token",
		Expires:       time.Now().Add(-100 * time.Second).Round(time.Millisecond),
	}

	ctx := context.Background()
	if err := store.Insert(ctx, &inserted); err != nil {
		t.Fatal(err)
	}

	if _, err := store.FindByID(ctx, inserted.CredentialsID); !errors.Is(err, tokens.ErrNotFound) {
		t.Fatalf("expected %q, got %q", tokens.ErrNotFound, err)
	}
}
