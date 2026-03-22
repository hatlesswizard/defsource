//go:build sqlite_fts5 || fts5

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	defsource "github.com/hatlesswizard/defsource"
	"github.com/hatlesswizard/defsource/internal/server"
)

func main() {
	dbPath := flag.String("db", "./data/defsource.db", "Path to SQLite database")
	addr := flag.String("addr", ":8080", "Server listen address")
	flag.Parse()

	corsOrigin := os.Getenv("DEFSOURCE_CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "*"
	}

	client, err := defsource.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

	srv := server.New(client, *addr, corsOrigin)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("defSource server listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
