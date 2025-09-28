package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/npezzotti/gophoto/config"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
	"github.com/npezzotti/gophoto/web"
	"github.com/npezzotti/gophoto/workers"
)

func main() {
	cfg, err := config.LoadConfigFromEnv()
	if err != nil {
		log.Fatalln("error generating config:", err)
	}

	dbConn, err := connectPostgres(cfg.DatabaseSource)
	if err != nil {
		log.Fatalln("error connecting to db:", err)
	}
	defer dbConn.Close()

	if err = db.Migrate("file://db/migrations", dbConn); err != nil {
		log.Fatalln("failed running migrations:", err)
	}

	querier := db.New(dbConn)

	photoStore, err := store.NewStore(cfg)
	if err != nil {
		log.Fatal("error creating store:", err)
	}

	redisClient, err := createRedisClient(cfg.RedisAddress)
	if err != nil {
		log.Fatal("error connecting to redis:", err)
	}
	defer redisClient.Close()

	ts, err := web.NewTemplateCache()
	if err != nil {
		log.Fatal("error creating template cache:", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(dbConn)
	gob.Register(web.Flash{})

	app := web.NewApplication(redisClient, cfg, sessionManager, querier, photoStore, ts)

	storageCleanerWorker := workers.NewStorageCleanerWorker(querier, photoStore, app.InfoLog, workers.FrequencyFifteenMin)
	storageCleanerWorker.Run()

	photoProcessorWorker, err := workers.NewPhotoProcessorWorker(redisClient, cfg, querier, photoStore, app.InfoLog)
	if err != nil {
		log.Fatal("error creating photo processor worker:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	photoProcessorWorker.Run()

	errChan := make(chan error)
	go func() {
		app.InfoLog.Printf("starting server on %s", cfg.HttpServerAddr)
		errChan <- app.Start()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("received signal, shutting down")
	case <-errChan:
		log.Println("error while running server")
	}

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
		app.InfoLog.Println("stopping worker")
		photoProcessorWorker.Stop()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		app.InfoLog.Println("stopping server")
		if err := app.Shutdown(ctx); err != nil {
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

func connectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func createRedisClient(addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
