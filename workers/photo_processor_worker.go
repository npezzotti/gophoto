package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/config"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
	"github.com/redis/go-redis/v9"
)

type PhotoProcessingJob struct {
	PhotoID int32
}

type PhotoProcessorWorker struct {
	redisClient *redis.Client
	imageOpts   []ImageOpts
	baseURL     *url.URL
	db          *db.Queries
	store       store.Store
	log         *log.Logger
	stopChan    chan struct{}
	doneChan    chan bool
}

type ImageOpts struct {
	Suffix  string
	Width   int
	Height  int
	Quality int
	Type    bimg.ImageType
}

func NewPhotoProcessorWorker(redisClient *redis.Client, cfg *config.Config, db *db.Queries, s store.Store, l *log.Logger) (*PhotoProcessorWorker, error) {
	ppw := &PhotoProcessorWorker{
		redisClient: redisClient,
		db:          db,
		store:       s,
		log:         l,
		imageOpts: []ImageOpts{
			{Suffix: string(store.FileSuffixThumbnail), Width: 300, Height: 300, Quality: 80, Type: bimg.WEBP},
			{Suffix: string(store.FileSuffixLarge), Width: 1920, Height: 1080, Quality: 90, Type: bimg.WEBP},
		},
		stopChan: make(chan struct{}),
		doneChan: make(chan bool, 1),
	}

	if cfg.StorageType == config.StorageTypeDisk {
		// If using local file storage, set the internal URL for downloading photos
		baseURL, err := url.Parse("http://" + cfg.HttpServerAddr)
		if err != nil {
			return nil, fmt.Errorf("error parsing base URL: %w", err)
		}
		ppw.baseURL = baseURL
	}

	return ppw, nil
}

func (ppw *PhotoProcessorWorker) Run() {
	ppw.log.Println("starting photo processor worker")

	// Subscribe to the Redis channel for photo processing jobs
	jobsChan := subscribeToQueue(ppw.redisClient, PhotoProcessingQueue)

	go func() {
		for {
			select {
			case msg := <-jobsChan:
				if msg == nil {
					continue
				}

				if err := ppw.handleJob(msg); err != nil {
					ppw.log.Println("error handling job:", err)
				}
			case <-ppw.stopChan:
				ppw.log.Println("stopping photo processor worker")
				select {
				case ppw.doneChan <- true:
				default:
				}
				return
			}
		}
	}()
}

// handleJob processes a single photo processing job message from the Redis queue.
func (ppw *PhotoProcessorWorker) handleJob(msg *redis.Message) error {
	ppw.log.Println("starting photo processing job")
	defer ppw.log.Println("finished photo processing job")

	var processingJob PhotoProcessingJob
	if err := json.Unmarshal([]byte(msg.Payload), &processingJob); err != nil {
		return fmt.Errorf("error unmarshalling message payload %q: %w", msg.Payload, err)
	}

	if err := ppw.processPhoto(processingJob.PhotoID); err != nil {
		return fmt.Errorf("error processing photo ID %d: %w", processingJob.PhotoID, err)
	}
	return nil
}

func (ppw *PhotoProcessorWorker) updatePhotoStatus(photo db.Photo, status db.PhotoStatus) error {
	return ppw.db.UpdatePhotoStatus(context.Background(), db.UpdatePhotoStatusParams{
		ID:        photo.ID,
		Status:    status,
		UpdatedAt: time.Now(),
	})
}

func (ppw *PhotoProcessorWorker) processPhoto(photoId int32) error {
	ppw.log.Printf("starting photo processing job for photo ID %d", photoId)

	photo, err := ppw.db.GetPhoto(context.Background(), photoId)
	if err != nil {
		return fmt.Errorf("error getting photo from database: %v", err)
	}

	var processingErr error
	defer func() {
		if processingErr != nil {
			ppw.updatePhotoStatus(photo, db.PhotoStatusErrored)
		}
	}()
	photoBytes, err := ppw.downloadOriginal(photo)
	if err != nil {
		processingErr = err
		return fmt.Errorf("error downloading original photo: %v", err)
	}

	meta, err := bimg.NewImage(photoBytes).Metadata()
	if err != nil {
		processingErr = err
		return fmt.Errorf("error getting image metadata: %v", err)
	}

	for _, opts := range ppw.imageOpts {
		imageOpts := bimg.Options{
			Width:   opts.Width,
			Height:  opts.Height,
			Quality: opts.Quality,
			Type:    opts.Type,
		}

		widthRatio := float64(meta.Size.Width) / float64(imageOpts.Width)
		heightRatio := float64(meta.Size.Height) / float64(imageOpts.Height)

		if widthRatio < heightRatio {
			imageOpts.Height = 0
		} else {
			imageOpts.Width = 0
		}

		processedImg, err := bimg.NewImage(photoBytes).Process(imageOpts)
		if err != nil {
			processingErr = err
			ppw.log.Printf("error processing image for %s: %v", opts.Suffix, err)
			continue
		}

		if err := ppw.store.Write(context.Background(), photo.Key+opts.Suffix, bytes.NewReader(processedImg)); err != nil {
			processingErr = err
			ppw.log.Printf("error writing %s image to store: %v", opts.Suffix, err)
			continue
		}
	}

	if err := ppw.updatePhotoStatus(photo, db.PhotoStatusProcessed); err != nil {
		processingErr = err
		ppw.log.Printf("error updating photo %d status: %v", photoId, err)
	}
	return nil
}

func (ppw *PhotoProcessorWorker) downloadOriginal(photo db.Photo) ([]byte, error) {
	photoURL, err := ppw.store.Read(context.Background(), photo.Key+string(store.FileSuffixOriginal))
	if err != nil {
		return nil, fmt.Errorf("error reading photo from store: %v", err)
	}

	if ppw.baseURL != nil {
		// If using local file storage, prepend the base URL to the photo URL
		photoURL = ppw.baseURL.String() + photoURL
	}

	resp, err := http.Get(photoURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching photo from URL: %v", err)
	}
	defer resp.Body.Close()

	buffer, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading photo response body: %v", err)
	}
	return buffer, nil
}

func (ppw *PhotoProcessorWorker) Stop() {
	close(ppw.stopChan)
	<-ppw.doneChan
}
