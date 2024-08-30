package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/pilatescompletebot/internal/credentials"
	httpx "github.com/pilatescompletebot/internal/http"
	"github.com/pilatescompletebot/internal/keys"
	"github.com/pilatescompletebot/internal/pilatescomplete"
	"github.com/pilatescompletebot/internal/tokens"
)

func main() {
	addr := flag.String("address", ":http", "http address to listen to")
	dbPath := flag.String("database-path", "pilatedcomplete.db", "path to the database")
	key := flag.String("encryption-key", "please-change-me", "encryption key for the database")
	flag.Parse()

	encryptionKey, err := keys.ParseKey([]byte(*key))
	if err != nil {
		log.Fatalf("[ERROR] encryption-key: %s", err)
	}

	db, err := badger.Open(badger.DefaultOptions(*dbPath))
	if err != nil {
		log.Fatalf("[ERROR] db %s", err)
	}

	credentialsStore := credentials.NewStore(db, encryptionKey)
	tokensStore := tokens.NewStore(db, encryptionKey)
	client := pilatescomplete.NewClient()
	htmlHandler := httpx.Handler(client, tokensStore, credentialsStore)

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
