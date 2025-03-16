package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api   *tgbotapi.BotAPI
	store *Store
}

func NewBot(store *Store, token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		api:   api,
		store: store,
	}, nil
}

func (b *Bot) Broadcast(ctx context.Context, message string) error {
	chats, err := b.store.ListChats(ctx)
	if err != nil {
		return fmt.Errorf("list chats: %w", err)
	}
	for _, chat := range chats {
		msg := tgbotapi.NewMessage(chat.ID, message)
		if _, err := b.api.Send(msg); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}
	return nil
}

func (b *Bot) Listen(ctx context.Context) error {
	offset, err := b.store.GetUpdatesOffset(ctx)
	if err != nil {
		return fmt.Errorf("get updates offset: %w", err)
	}
	updates := b.api.GetUpdatesChan(tgbotapi.UpdateConfig{Offset: offset})
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "stopping listening for telegram updates")
			return nil
		case update := <-updates:
			if update.Message.IsCommand() {
				if err := b.handleCommand(ctx, update.Message); err != nil {
					slog.ErrorContext(ctx, "handle command", "error", err)
				}
			}

			if err := b.store.SetUpdatesOffset(ctx, update.UpdateID+1); err != nil {
				slog.ErrorContext(ctx, "set updates offset", "error", err)
			}
		}
	}
}

func (b *Bot) handleCommand(ctx context.Context, message *tgbotapi.Message) error {
	switch message.Command() {
	case "start":
		return b.handleStart(ctx, message)
	default:
		return nil
	}
}

func (b *Bot) handleStart(ctx context.Context, message *tgbotapi.Message) error {
	chat := Chat{
		ID:        message.Chat.ID,
		FirstName: message.Chat.FirstName,
	}
	if err := b.store.InsertChat(ctx, &chat); err != nil {
		return fmt.Errorf("insert chat: %w", err)
	}
	return nil
}
