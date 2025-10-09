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
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/store"
	"github.com/npezzotti/gophoto/internal/web"
	"github.com/npezzotti/gophoto/internal/workers"
	"github.com/npezzotti/gophoto/pkg/template"
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

	if err = db.Migrate(dbConn); err != nil {
		log.Fatalln("failed running migrations:", err)
	}

	querier := db.New(dbConn)

	var photoStore store.Store
	switch cfg.StorageType {
	case config.StorageTypeDisk:
		photoStore, err = store.NewFileStore(cfg.BaseDir, cfg.SigningKey)
	case config.StorageTypeS3:
		photoStore = store.NewS3Store(cfg.BucketName)
	default:
		log.Fatal("storage type not supported")
	}
	if err != nil {
		log.Fatalln("error creating store:", err)
	}

	redisClient, err := createRedisClient(cfg.RedisAddress)
	if err != nil {
		log.Fatal("error connecting to redis:", err)
	}
	defer redisClient.Close()

	tc, err := template.NewTemplateCache()
	if err != nil {
		log.Fatal("error creating template cache:", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(dbConn)
	gob.Register(web.Flash{})

	app := web.NewApplication(redisClient, cfg, sessionManager, querier, photoStore, tc)

	storageCleanerWorker := workers.NewStorageCleanerWorker(querier, photoStore, app.InfoLog, workers.DefaultFrequency)
	storageCleanerWorker.Run()

	photoProcessorWorker := workers.NewPhotoProcessorWorker(redisClient, cfg, querier, photoStore, app.InfoLog)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

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
