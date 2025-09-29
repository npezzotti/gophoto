package workers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
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

type TickerFrequency time.Duration

const (
	FrequencyFifteenMin = TickerFrequency(1 * time.Minute)
)

func NewStorageCleanerWorker(db *db.Queries, store store.Store, logger *log.Logger, frequency TickerFrequency) StorageCleanerWorker {
	return StorageCleanerWorker{
		db:       db,
		store:    store,
		log:      logger,
		ticker:   time.NewTicker(time.Duration(frequency)),
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
	photos, err := scw.db.GetOrphanedPhotos(context.Background())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			scw.log.Println("error getting files:", err)
		}
		return
	}

	scw.log.Printf("found %d orphaned photos to delete", len(photos))

	for _, photo := range photos {
		for _, suffix := range []store.FileSuffix{"", store.FileSuffixThumbnail, store.FileSuffixLarge} {
			key := photo.Key + string(suffix)
			if err := scw.store.Delete(context.Background(), key); err != nil {
				if !errors.Is(err, store.ErrNotExist) {
					scw.log.Printf("error deleting file with key %s: %s", key, err.Error())
					return
				}
			}
		}

		if err := scw.db.DeletePhoto(context.Background(), photo.ID); err != nil {
			scw.log.Printf("error deleting photo %d from database: %s", photo.ID, err.Error())
		}
	}

	scw.log.Println("finished storage cleanup job")
}

func (scw *StorageCleanerWorker) Stop() {
	scw.ticker.Stop()
	close(scw.stopChan)
	<-scw.doneChan
}
