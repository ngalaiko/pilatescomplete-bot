package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescomplete-bot/internal/authentication"
	"github.com/pilatescomplete-bot/internal/calendars"
	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/events"
	httpx "github.com/pilatescomplete-bot/internal/http"
	"github.com/pilatescomplete-bot/internal/http/static"
	"github.com/pilatescomplete-bot/internal/http/templates"
	"github.com/pilatescomplete-bot/internal/jobs"
	"github.com/pilatescomplete-bot/internal/keys"
	"github.com/pilatescomplete-bot/internal/migrations"
	"github.com/pilatescomplete-bot/internal/notifications"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/statistics"
	"github.com/pilatescomplete-bot/internal/telegram"
	"github.com/pilatescomplete-bot/internal/tokens"
	"golang.org/x/sync/errgroup"
)

func main() {
	addr := flag.String("address", ":http", "http address to listen to")
	dbPath := flag.String("database-path", "pilatedcomplete.db", "path to the database")
	key := flag.String("encryption-key", "please-change-me", "encryption key for the database")
	watch := flag.Bool("watch", false, "if true, will serve from filesystem")
	telegramBotToken := flag.String("telegram-bot-token", "", "Telegram bot token")
	flag.Parse()

	if envKey := os.Getenv("ENCRYPTION_KEY"); envKey != "" {
		key = &envKey
	}

	if envKey := os.Getenv("TELEGRAM_BOT_TOKEN"); envKey != "" {
		telegramBotToken = &envKey
	}

	encryptionKey, err := keys.ParseKey([]byte(*key))
	if err != nil {
		log.Fatalf("[ERROR] encryption-key: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := badger.Open(badger.DefaultOptions(*dbPath))
	if err != nil {
		log.Fatalf("[ERROR] db: %s", err)
	}

	if err := migrations.Run(db); err != nil {
		slog.ErrorContext(ctx, "migrations", "error", err)
		os.Exit(1)
	}

	var renderer templates.Renderer
	var staticHandler http.Handler
	if *watch {
		renderer = templates.NewFilesystemTemplates("./internal/http/templates")
		staticHandler = static.NewFilesystemHandler("./internal/http/static/files")
	} else {
		renderer = templates.NewEmbedTemplates()
		staticHandler = static.NewEmbedHandler()
	}

	credentialsStore := credentials.NewStore(db, encryptionKey)
	tokensStore := tokens.NewStore(db, encryptionKey)
	jobsStore := jobs.NewStore(db)
	apiClient := pilatescomplete.NewAPIClient()
	authenticationService := authentication.NewService(tokensStore, credentialsStore, apiClient)
	eventsService := events.NewService(jobsStore, apiClient)

	var handler slog.Handler
	handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: new(slog.LevelVar),
	})

	errGroup := errgroup.Group{}
	scheduler := jobs.NewScheduler(jobsStore, apiClient, authenticationService)
	if *telegramBotToken != "" {
		telegramStore := telegram.NewStore(db)
		telegramBot, err := telegram.NewBot(authenticationService, eventsService, telegramStore, *telegramBotToken)
		if err != nil {
			log.Fatalf("[ERROR] telegram bot: %s", err)
		}

		handler = telegram.NewSlogHandler(telegramBot, handler)
		scheduler.OnJobFailed(func(ctx context.Context, job *jobs.Job) {
			if job.BookEvent != nil {
				if err := telegramBot.BroadcastBookEventFailed(ctx, job); err != nil {
					slog.ErrorContext(ctx, "broadcast book event failed", "error", err)
				}
			}
		})
		scheduler.OnJobSucceeded(func(ctx context.Context, job *jobs.Job) {
			if job.BookEvent != nil {
				if err := telegramBot.BroadcastEventBooked(ctx, job); err != nil {
					slog.ErrorContext(ctx, "broadcast event booked", "error", err)
				}
			}
		})

		errGroup.Go(func() error {
			if err := telegramBot.Listen(ctx); err != nil {
				return fmt.Errorf("telegram bot listen: %w", err)
			}
			return nil
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	calendarsStore := calendars.NewStore(db)
	calendarsService := calendars.NewService(calendarsStore, authenticationService, eventsService)
	notificationsService := notifications.NewService(apiClient)
	statisticsService := statistics.NewService(notificationsService)
	if err := scheduler.Init(ctx); err != nil {
		log.Fatalf("[ERROR] scheduler init: %s", err)
		os.Exit(1)
	}
	htmlHandler := httpx.Handler(
		renderer,
		staticHandler,
		apiClient,
		tokensStore,
		credentialsStore,
		authenticationService,
		eventsService,
		scheduler,
		calendarsService,
		statisticsService,
	)

	httpServer := http.Server{
		Handler: htmlHandler,
	}

	// Wait for shut down in a separate goroutine.
	errCh := make(chan error)
	go func() {
		shutdownCh := make(chan os.Signal, 1)
		signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
		sig := <-shutdownCh

		log.Printf("[INFO] received %s, shutting down", sig)
		cancel()

		shutdownTimeout := 15 * time.Second
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		errCh <- httpServer.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		slog.ErrorContext(ctx, "listen", "error", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "listen", "address", ln.Addr())

	errGroup.Go(func() error {
		if err := httpServer.Serve(ln); err != http.ErrServerClosed {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})
	if err := errGroup.Wait(); err != nil {
		slog.ErrorContext(ctx, "error", "error", err)
		os.Exit(1)
	}

	if err := <-errCh; err != nil {
		slog.ErrorContext(ctx, "shutdown", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "application stopped")
}
