package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/npezzotti/gophoto/config"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
	"github.com/npezzotti/gophoto/workers"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalln("error generating config:", err)
	}

	dbConn, err := Open(cfg.DatabaseSource)
	if err != nil {
		log.Fatalln("error connecting to db:", err)
	}
	defer dbConn.Close()

	if err = Migrate("file://db/migrations", dbConn); err != nil {
		log.Fatalln("failed running migrations:", err)
	}

	db := db.New(dbConn)

	photoStore, err := store.NewStore(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ts, err := NewTemplateCache()
	if err != nil {
		log.Fatal("error creating template cache:", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(dbConn)
	gob.Register(Flash{})

	app := NewApplication(cfg, sessionManager, db, photoStore, ts)

	storageCleanerWorker := workers.NewStorageCleanerWorker(app.database, app.store, app.InfoLog, workers.FrequencyFifteenMin)
	storageCleanerWorker.Start()

	srv := &http.Server{
		Addr:     cfg.HttpServerAddr,
		Handler:  setupMiddleware(app.mux, app.sessionManager.LoadAndSave, noSurf, app.authenticate),
		ErrorLog: app.ErrorLog,
	}

	errChan := make(chan error)

	go func() {
		app.InfoLog.Printf("starting server on %s", cfg.HttpServerAddr)
		errChan <- srv.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("received signal, shutting down")
	case <-errChan:
		log.Println("error while running server")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		app.InfoLog.Println("stopping worker")
		storageCleanerWorker.Stop()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		app.InfoLog.Println("stopping server")
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("error shutting down server: %v", err)
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case sig := <-sigChan:
		log.Printf("received second signal %s, aborting", sig)
	case <-doneChan:
		log.Println("graceful shutdown complete")
	case <-ctx.Done():
		log.Fatal("timed out before graceful shutdown finished")
	}
}

func Migrate(source string, db *sql.DB) error {
	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("error creating driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(source, "postgres", databaseDriver)
	if err != nil {
		return fmt.Errorf("error creating migrate instance: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error running migrations driver: %w", err)
		}
	}
	return nil
}

func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
