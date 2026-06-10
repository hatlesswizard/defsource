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
	"github.com/hatlesswizard/defsource/internal/config"
	"github.com/hatlesswizard/defsource/internal/server"
	_ "github.com/hatlesswizard/defsource/internal/source/allsources"
)

func main() {
	// Load env-var defaults first; CLI flags override them when explicitly passed.
	// Precedence: explicit CLI flag > env var > built-in default (see config.Load).
	cfg := config.Load()

	dbPath := flag.String("db", cfg.DBPath, "Path to SQLite database")
	addr := flag.String("addr", cfg.ServerAddr, "Server listen address")
	flag.Parse()

	client, err := defsource.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

	srv := server.New(client, *addr, cfg.CORSOrigin)

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
