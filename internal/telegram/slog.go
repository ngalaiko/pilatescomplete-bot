package telegram

import (
	"context"
	"fmt"
	"log/slog"
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
	return l >= slog.LevelWarn
}

// Handle intercepts logs and sends error logs to Telegram.
func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.bot.BroadcastSlogRecord(ctx, r); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
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
