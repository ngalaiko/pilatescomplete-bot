package main

import (
	"context"
	"flag"
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
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

func main() {
	addr := flag.String("address", ":http", "http address to listen to")
	dbPath := flag.String("database-path", "pilatedcomplete.db", "path to the database")
	key := flag.String("encryption-key", "please-change-me", "encryption key for the database")
	watch := flag.Bool("watch", false, "if true, will serve from filesystem")
	flag.Parse()

	if envKey := os.Getenv("ENCRYPTION_KEY"); envKey != "" {
		key = &envKey
	}

	encryptionKey, err := keys.ParseKey([]byte(*key))
	if err != nil {
		log.Fatalf("[ERROR] encryption-key: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: new(slog.LevelVar),
	}))

	db, err := badger.Open(badger.DefaultOptions(*dbPath))
	if err != nil {
		log.Fatalf("[ERROR] db: %s", err)
	}

	if err := migrations.Run(logger, db); err != nil {
		log.Fatalf("[ERROR] db migratons: %s", err)
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
	apiClient := pilatescomplete.NewAPIClient(logger)
	authenticationService := authentication.NewService(tokensStore, credentialsStore, apiClient)
	eventsService := events.NewService(jobsStore, apiClient)
	scheduler := jobs.NewScheduler(logger, jobsStore, apiClient, authenticationService)
	calendarsStore := calendars.NewStore(db)
	calendarsService := calendars.NewService(calendarsStore, authenticationService, eventsService)
	if err := scheduler.Init(ctx); err != nil {
		log.Fatalf("[ERROR] init scheduler: %s", err)
	}
	htmlHandler := httpx.Handler(
		logger,
		renderer,
		staticHandler,
		apiClient,
		tokensStore,
		credentialsStore,
		authenticationService,
		eventsService,
		scheduler,
		calendarsService,
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

		shutdownTimeout := 15 * time.Second
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		errCh <- httpServer.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("[ERROR] tcp: %s", err)
	}
	log.Printf("[INFO] listening on %s", ln.Addr())

	if err := httpServer.Serve(ln); err != http.ErrServerClosed {
		log.Printf("[ERROR] http serve: %s", err)
	}

	if err := <-errCh; err != nil {
		log.Printf("[ERROR] error during shutdown: %s", err)
	}

	log.Printf("[INFO] application stopped")
}
