package workers

import (
	"bytes"
	"context"
	"database/sql"
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

type PhotoType string

const (
	PhotoTypeUserPhoto  PhotoType = "user_photo"
	PhotoTypeProfilePic PhotoType = "profile_picture"
)

type ImageOpts struct {
	Variant db.PhotoVariant
	Width   int
	Height  int
	Quality int
	Type    bimg.ImageType
}

var (
	UserPhotoSizes []ImageOpts = []ImageOpts{
		{Variant: db.PhotoVariantThumb, Width: 300, Height: 300, Quality: 80, Type: bimg.WEBP},
		{Variant: db.PhotoVariantLarge, Width: 1920, Height: 1080, Quality: 90, Type: bimg.WEBP},
	}

	ProfilePicSizes []ImageOpts = []ImageOpts{
		{Variant: db.PhotoVariantAvatar, Width: 100, Height: 100, Quality: 80, Type: bimg.WEBP},
		{Variant: db.PhotoVariantThumb, Width: 300, Height: 300, Quality: 80, Type: bimg.WEBP},
	}
)

type PhotoProcessingJob struct {
	Type    PhotoType
	PhotoID int32
	UserID  int32
}

type PhotoProcessorWorker struct {
	redisClient *redis.Client
	baseURL     *url.URL
	db          *db.Queries
	store       store.Store
	log         *log.Logger
	stopChan    chan struct{}
	doneChan    chan bool
}

func NewPhotoProcessorWorker(redisClient *redis.Client, cfg *config.Config, db *db.Queries, s store.Store, l *log.Logger) (*PhotoProcessorWorker, error) {
	ppw := &PhotoProcessorWorker{
		redisClient: redisClient,
		db:          db,
		store:       s,
		log:         l,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan bool, 1),
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

	var sizes []ImageOpts
	switch processingJob.Type {
	case PhotoTypeUserPhoto:
		sizes = UserPhotoSizes
	case PhotoTypeProfilePic:
		sizes = ProfilePicSizes
	default:
		return fmt.Errorf("unknown photo type: %s", processingJob.Type)
	}

	if err := ppw.processPhoto(processingJob.PhotoID, sizes); err != nil {
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

func (ppw *PhotoProcessorWorker) processPhoto(photoId int32, sizes []ImageOpts) error {
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

	for _, size := range sizes {
		imageOpts := bimg.Options{
			Width:   size.Width,
			Height:  size.Height,
			Quality: size.Quality,
			Type:    size.Type,
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
			ppw.log.Printf("error processing %s image: %v", size.Variant, err)
			continue
		}

		photoMeta, err := ppw.db.CreatePhotoMetadata(context.Background(), db.CreatePhotoMetadataParams{
			PhotoID:  photo.ID,
			Variant:  size.Variant,
			FileSize: sql.NullInt64{Int64: int64(len(processedImg)), Valid: true},
			MimeType: "image/webp",
		})
		if err != nil {
			processingErr = err
			ppw.log.Printf("error creating photo metadata for %q: %v", size.Variant, err)
			continue
		}

		if err := ppw.store.Write(context.Background(), store.BuildPhotoPath(photo.Key, photoMeta.Variant), bytes.NewReader(processedImg)); err != nil {
			processingErr = err
			ppw.log.Printf("error writing %s image to store: %v", size.Variant, err)
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
	photoFile := store.BuildPhotoPath(photo.Key, db.PhotoVariantOriginal)
	photoURL, err := ppw.store.GenerateURL(context.Background(), photoFile)
	if err != nil {
		return nil, fmt.Errorf("error generating photo URL: %v", err)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching photo, status code: %d", resp.StatusCode)
	}

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
