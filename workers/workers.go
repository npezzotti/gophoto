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

type Worker interface {
	Start()
	Perform()
	Stop()
}

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
	FrequencyFifteenMin = TickerFrequency(15 * time.Minute)
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

func (scw *StorageCleanerWorker) Start() {
	scw.log.Println("starting storage cleaner worker")
	go func() {
		for {
			select {
			case <-scw.stopChan:
				scw.log.Println("received shutdown signal")
				scw.doneChan <- true
				return
			case <-scw.ticker.C:
				scw.Perform()
			}
		}
	}()
}

func (scw *StorageCleanerWorker) Perform() {
	scw.log.Println("starting storage cleanup job")
	photos, err := scw.db.GetOrphanedPhotos(context.Background())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			scw.log.Println("error getting files:", err)
		}
		return
	}

	for _, photo := range photos {
		if err := scw.store.Delete(context.Background(), photo.Key); err != nil {
			if !errors.Is(err, store.ErrNotExist) {
				scw.log.Printf("error deleting file with key %s: %s", photo.Key, err.Error())
			}
		}

		scw.db.DeletePhoto(context.Background(), photo.ID)
	}

	scw.log.Println("finished storage cleanup job")
}

func (scw *StorageCleanerWorker) Stop() {
	scw.ticker.Stop()
	close(scw.stopChan)
	<-scw.doneChan
}
