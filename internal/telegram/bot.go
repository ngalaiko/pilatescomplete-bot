package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/events"
	"github.com/pilatescomplete-bot/internal/jobs"
)

type Bot struct {
	authenticationService *authentication.Service
	eventsService         *events.Service

	api   *tgbotapi.BotAPI
	store *Store
}

func NewBot(
	authenticationService *authentication.Service,
	eventsService *events.Service,
	store *Store,
	token string,
) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		authenticationService: authenticationService,
		eventsService:         eventsService,
		api:                   api,
		store:                 store,
	}, nil
}

func (b *Bot) BroadcastBookEventFailed(ctx context.Context, job *jobs.Job) error {
	ctx, err := b.authenticationService.AuthenticateContext(ctx, job.BookEvent.CredentialsID)
	if err != nil {
		return fmt.Errorf("authenticate context: %w", err)
	}
	event, err := b.eventsService.GetEvent(ctx, job.BookEvent.EventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}
	msg := &tgbotapi.MessageConfig{
		Text: fmt.Sprintf("Failed to book %s on %s: %s", event.DisplayName, event.StartTime.Format("Monday Jan 06 at 15:04"), job.Errors[len(job.Errors)-1]),
		Entities: []tgbotapi.MessageEntity{
			{
				Type:   "bold",
				Offset: 15,
				Length: len(event.DisplayName),
			},
		},
	}
	if err := b.broadcast(ctx, msg); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
	return nil
}

func (b *Bot) BroadcastEventBooked(ctx context.Context, job *jobs.Job) error {
	ctx, err := b.authenticationService.AuthenticateContext(ctx, job.BookEvent.CredentialsID)
	if err != nil {
		return fmt.Errorf("authenticate context: %w", err)
	}
	event, err := b.eventsService.GetEvent(ctx, job.BookEvent.EventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}
	msg := &tgbotapi.MessageConfig{
		Text: fmt.Sprintf("Booked %s on %s", event.DisplayName, event.StartTime.Format("Monday Jan 06 at 15:04")),
		Entities: []tgbotapi.MessageEntity{
			{
				Type:   "bold",
				Offset: 7,
				Length: len(event.DisplayName),
			},
		},
	}
	if err := b.broadcast(ctx, msg); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
	return nil
}

func (b *Bot) BroadcastSlogRecord(ctx context.Context, r slog.Record) error {
	text := strings.Builder{}
	text.WriteString(fmt.Sprintf("[%s] ", r.Level))
	text.WriteString(r.Message)
	r.Attrs(func(attr slog.Attr) bool {
		text.WriteString(fmt.Sprintf("\n%s: %v", attr.Key, attr.Value))
		return true
	})

	msg := &tgbotapi.MessageConfig{
		Text: text.String(),
		Entities: []tgbotapi.MessageEntity{
			{
				Type:   "code",
				Offset: 0,
				Length: text.Len(),
			},
			{
				Type:   "bold",
				Offset: 1,
				Length: len(r.Level.String()),
			},
		},
	}

	if err := b.broadcast(ctx, msg); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
	return nil
}

func (b *Bot) broadcast(ctx context.Context, msg *tgbotapi.MessageConfig) error {
	chats, err := b.store.ListChats(ctx)
	if err != nil {
		return fmt.Errorf("list chats: %w", err)
	}
	for _, chat := range chats {
		msg.BaseChat.ChatID = chat.ID
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
