package telegram

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func TestStore(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)

	chats := []Chat{
		{ID: 1, FirstName: "chat1"},
		{ID: 2, FirstName: "chat2"},
	}

	ctx := context.Background()

	for _, chat := range chats {
		if err := store.InsertChat(ctx, &chat); err != nil {
			t.Fatalf("failed to insert chat: %v", err)
		}
	}

	listed, err := store.ListChats(ctx)
	if err != nil {
		t.Fatalf("failed to list chats: %v", err)
	}

	if len(listed) != len(chats) {
		t.Fatalf("expected %d chats, got %d", len(chats), len(listed))
	}

	for i, chat := range chats {
		if chat != listed[i] {
			t.Fatalf("expected %v, got %v", chat, listed[i])
		}
	}
}

func TestUpdatesOffset(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)

	ctx := context.Background()

	if err := store.SetUpdatesOffset(ctx, 100); err != nil {
		t.Fatalf("failed to set updates offset: %v", err)
	}

	offset, err := store.GetUpdatesOffset(ctx)
	if err != nil {
		t.Fatalf("failed to get updates offset: %v", err)
	}

	if offset != 100 {
		t.Fatalf("expected 100, got %d", offset)
	}
}
