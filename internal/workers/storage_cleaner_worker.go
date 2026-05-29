package workers

import (
	"context"
	"errors"
	"time"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/store"
)

// The StorageCleanerWorker periodically checks for orphaned photos in the storage
// and deletes their backing files to free up space.
type StorageCleanerWorker struct {
	db       db.PhotoRepository
	store    store.Store
	log      *logging.Logger
	ticker   *time.Ticker
	stopChan chan struct{}
	doneChan chan bool
}

const (
	DefaultFrequency = 15 * time.Minute
	DefaultTimeLimit = 10 * time.Minute
)

func NewStorageCleanerWorker(db db.PhotoRepository, store store.Store, logger *logging.Logger, frequency time.Duration) StorageCleanerWorker {
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
	scw.log.Info("starting storage cleaner worker")
	go func() {
		for {
			select {
			case <-scw.stopChan:
				scw.log.Info("received shutdown signal")
				scw.doneChan <- true
				return
			case <-scw.ticker.C:
				scw.cleanStorage()
			}
		}
	}()
}

func (scw *StorageCleanerWorker) cleanStorage() {
	scw.log.Info("starting storage cleanup job")
	defer scw.log.Info("finished storage cleanup job")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeLimit)
	defer cancel()

	photos, err := scw.db.GetOrphanedPhotos(ctx)
	if err != nil {
		scw.log.Error("error getting files: %v", err)
		return
	}

	scw.log.Info("found %d orphaned photos to delete", len(photos))

	for _, photo := range photos {
		metadata, err := scw.db.GetPhotoMetadataByPhotoID(ctx, photo.ID)
		if err != nil {
			scw.log.Error("error getting metadata for photo %d: %v", photo.ID, err)
			continue
		}
		for _, m := range metadata {
			path, err := utils.BuildPhotoPathForVariant(photo.Key, m.Variant, utils.MimeType(m.MimeType))
			if err != nil {
				scw.log.Error("error building path for photo %d variant %s: %v", photo.ID, m.Variant, err)
				continue
			}
			if err := scw.store.Delete(ctx, path); err != nil {
				if !errors.Is(err, store.ErrNotExist) {
					scw.log.Error("error deleting file with key %s: %v", path, err)
					return
				}
			}
		}

		if err := scw.db.DeletePhoto(ctx, photo.ID); err != nil {
			scw.log.Error("error deleting photo %d from database: %v", photo.ID, err)
			return
		}

		// Check if the context has been cancelled or timed out after each iteration
		if err := ctx.Err(); err != nil {
			scw.log.Error("context error: %v", err)
			return
		}
	}
}

func (scw *StorageCleanerWorker) Stop() {
	scw.ticker.Stop()
	close(scw.stopChan)
	<-scw.doneChan
}
