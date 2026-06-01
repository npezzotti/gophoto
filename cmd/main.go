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
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/service"
	"github.com/npezzotti/gophoto/internal/web"
	"github.com/npezzotti/gophoto/internal/workers"
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/store"
	"github.com/npezzotti/gophoto/pkg/template"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln("error running application:", err)
	}
}

func run() error {
	cfg, err := config.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("error generating config: %w", err)
	}

	// Connect to the PostgreSQL database and run migrations
	dbConn, err := connectPostgres(cfg.DatabaseSource)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer dbConn.Close()

	if err = db.Migrate(dbConn); err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	// Initialize the store based on the configuration
	var photoStore store.Store
	var storeErr error
	switch cfg.StorageType {
	case config.StorageTypeDisk:
		photoStore, storeErr = store.NewFileStore(cfg.BaseDir, cfg.SigningKey)
	case config.StorageTypeS3:
		photoStore, storeErr = store.NewS3Store(cfg.BucketName)
	default:
		storeErr = fmt.Errorf("storage type not supported")
	}
	if storeErr != nil {
		return fmt.Errorf("error creating store: %w", storeErr)
	}

	// Initialize the Redis client
	redisClient, err := createRedisClient(cfg.RedisAddress)
	if err != nil {
		return fmt.Errorf("error connecting to redis: %w", err)
	}
	defer redisClient.Close()

	// Initialize the template cache
	var tc template.TemplateCache
	if cfg.UseTemplateCache {
		tc, err = template.NewTemplateCache(web.PagesGlob, web.PartialsGlob, web.BaseTemplate)
		if err != nil {
			return fmt.Errorf("error creating template cache: %w", err)
		}
	}

	// Initialize the session manager with PostgreSQL store
	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(dbConn)

	// Register the Flash struct with gob for session serialization
	gob.Register(web.Flash{})

	// Initialize repositories, services, and the application
	repo := db.NewRepository(dbConn)
	logger := logging.NewLogger(os.Stderr, cfg.Debug)
	userService := service.NewUserService(repo, repo, photoStore, cfg, logger)
	photoService := service.NewPhotoService(repo, repo, photoStore, redisClient, logger)
	albumService := service.NewAlbumService(repo, repo, photoStore, cfg, logger)
	app := web.NewApplication(userService, albumService, photoService, cfg, sessionManager, tc, logger)

	storageCleanerWorker := workers.NewStorageCleanerWorker(repo, photoStore, logger, workers.DefaultFrequency)
	storageCleanerWorker.Run()

	photoProcessorWorker := workers.NewPhotoProcessorWorker(redisClient, cfg, repo, photoStore, logger)
	photoProcessorWorker.Run()

	errChan := make(chan error, 1)
	go func() {
		logger.Info("starting server on %s", cfg.HttpServerAddr)
		errChan <- app.Start()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	var runErr error
	select {
	case <-sigChan:
		logger.Info("received signal, shutting down")
	case err := <-errChan:
		if err != nil {
			runErr = fmt.Errorf("error running server: %w", err)
		}
	}

	doneChan := make(chan struct{})
	var wg sync.WaitGroup
	storageWorkerErrChan := make(chan error, 1)
	photoWorkerErrChan := make(chan error, 1)
	serverShutdownErrChan := make(chan error, 1)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	wg.Add(1)
	go func() {
		storageWorkerErrChan <- storageCleanerWorker.Stop(shutdownCtx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		photoWorkerErrChan <- photoProcessorWorker.Stop(shutdownCtx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := app.Shutdown(shutdownCtx); err != nil {
			serverShutdownErrChan <- fmt.Errorf("error shutting down server: %w", err)
			wg.Done()
			return
		}
		serverShutdownErrChan <- nil
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Drain error channel to get any pending server error before proceeding with shutdown
	select {
	case err := <-errChan:
		if err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("error running server: %w", err))
		}
	default:
	}

	if runErr != nil {
		return runErr
	}

	select {
	case sig := <-sigChan:
		return fmt.Errorf("shutdown aborted due to signal: %s", sig)
	case <-doneChan:
		componentErr := errors.Join(<-storageWorkerErrChan, <-photoWorkerErrChan, <-serverShutdownErrChan)
		if componentErr != nil {
			return componentErr
		}
		logger.Info("graceful shutdown complete")
		return nil
	case <-shutdownCtx.Done():
		return fmt.Errorf("timed out before graceful shutdown finished")
	}
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
