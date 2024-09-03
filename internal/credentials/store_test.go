package credentials

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescomplete-bot/internal/keys"
)

func Test(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key, err := keys.NewKey()
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(db, key)

	inserted := Credentials{
		ID:       "id",
		Login:    "login",
		Password: "password",
	}

	ctx := context.Background()
	if err := store.Insert(ctx, &inserted); err != nil {
		t.Fatal(err)
	}

	foundByID, err := store.FindByID(ctx, inserted.ID)
	if err != nil {
		t.Fatal(err)
	}

	if inserted != *foundByID {
		t.Fatal("inserted != found")
	}

	foundByLogin, err := store.FindByLogin(ctx, inserted.Login)
	if err != nil {
		t.Fatal(err)
	}

	if inserted != *foundByLogin {
		t.Fatal("inserted.ID != foundByLogin.ID")
	}
}
