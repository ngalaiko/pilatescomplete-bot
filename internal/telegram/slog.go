package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

var _ slog.Handler = &SlogHandler{}

type SlogHandler struct {
	bot  *Bot
	next slog.Handler
	mu   sync.Mutex
}

func NewSlogHandler(bot *Bot, next slog.Handler) *SlogHandler {
	return &SlogHandler{
		bot:  bot,
		next: next,
	}
}

func (h *SlogHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= slog.LevelInfo
}

// Handle intercepts logs and sends error logs to Telegram.
func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	message := strings.Builder{}
	message.WriteString(fmt.Sprintf("[%s] ", r.Level))
	message.WriteString(r.Message)
	r.Attrs(func(attr slog.Attr) bool {
		message.WriteString(fmt.Sprintf("\n%s: %v", attr.Key, attr.Value))
		return true
	})
	if err := h.bot.Broadcast(ctx, message.String()); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}

	// Pass the log to the next handler (e.g., JSON logger)
	if h.next != nil {
		return h.next.Handle(ctx, r)
	}
	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return h
}
