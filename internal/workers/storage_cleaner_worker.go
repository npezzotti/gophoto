package workers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/store"
)

// The StorageCleanerWorker periodically checks for orphaned photos in the storage
// and deletes their backing files to free up space.
type StorageCleanerWorker struct {
	db       *db.Queries
	store    store.Store
	log      *log.Logger
	ticker   *time.Ticker
	stopChan chan struct{}
	doneChan chan bool
}

const (
	DefaultFrequency = 15 * time.Minute
	DefaultTimeLimit = 10 * time.Minute
)

func NewStorageCleanerWorker(db *db.Queries, store store.Store, logger *log.Logger, frequency time.Duration) StorageCleanerWorker {
	return StorageCleanerWorker{
		db:       db,
		store:    store,
		log:      logger,
		ticker:   time.NewTicker(frequency),
		stopChan: make(chan struct{}),
		doneChan: make(chan bool, 1),
	}
}

func (scw *StorageCleanerWorker) Run() {
	scw.log.Println("starting storage cleaner worker")
	go func() {
		for {
			select {
			case <-scw.stopChan:
				scw.log.Println("received shutdown signal")
				scw.doneChan <- true
				return
			case <-scw.ticker.C:
				scw.cleanStorage()
			}
		}
	}()
}

func (scw *StorageCleanerWorker) cleanStorage() {
	scw.log.Println("starting storage cleanup job")
	defer scw.log.Println("finished storage cleanup job")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeLimit)
	defer cancel()

	photos, err := scw.db.GetOrphanedPhotos(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			scw.log.Println("error getting files:", err)
		}
		return
	}

	scw.log.Printf("found %d orphaned photos to delete", len(photos))

	for _, photo := range photos {
		metadata, err := scw.db.GetPhotoMetadataByPhotoID(ctx, photo.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			scw.log.Printf("error getting metadata for photo %d: %s", photo.ID, err.Error())
			continue
		}
		for _, m := range metadata {
			path := utils.BuildPhotoPath(photo.Key, m.Variant)
			if err := scw.store.Delete(ctx, path); err != nil {
				if !errors.Is(err, store.ErrNotExist) {
					scw.log.Printf("error deleting file with key %s: %s", path, err.Error())
					return
				}
			}
		}

		if err := scw.db.DeletePhoto(ctx, photo.ID); err != nil {
			scw.log.Printf("error deleting photo %d from database: %s", photo.ID, err.Error())
			return
		}

		// Check if the context has been cancelled or timed out after each iteration
		if err := ctx.Err(); err != nil {
			scw.log.Println("context error:", err)
			return
		}
	}
}

func (scw *StorageCleanerWorker) Stop() {
	scw.ticker.Stop()
	close(scw.stopChan)
	<-scw.doneChan
}
