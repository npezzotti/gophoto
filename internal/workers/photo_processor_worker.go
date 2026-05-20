package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/store"
	"github.com/redis/go-redis/v9"
)

type JobType string

const (
	JobTypeAlbumPhoto JobType = "album_photo"
	JobTypeUserPhoto  JobType = "user_photo"
)

type PhotoProcessingJob struct {
	Type    JobType
	PhotoID int32
	UserID  int32
}

type ImageOpts struct {
	Variant domain.PhotoVariant
	Width   int
	Height  int
	Quality int
	Type    bimg.ImageType
}

var (
	AlbumPhotoSizes []ImageOpts = []ImageOpts{
		{Variant: domain.PhotoVariantThumb, Width: 150, Height: 150, Quality: 70, Type: bimg.WEBP},
		{Variant: domain.PhotoVariantSmall, Width: 400, Height: 300, Quality: 80, Type: bimg.WEBP},
		{Variant: domain.PhotoVariantMedium, Width: 800, Height: 600, Quality: 80, Type: bimg.WEBP},
		{Variant: domain.PhotoVariantLarge, Width: 1920, Height: 1080, Quality: 90, Type: bimg.WEBP},
	}

	ProfilePicSizes []ImageOpts = []ImageOpts{
		{Variant: domain.PhotoVariantAvatar, Width: 100, Height: 100, Quality: 70, Type: bimg.WEBP},
		{Variant: domain.PhotoVariantSmall, Width: 400, Height: 300, Quality: 80, Type: bimg.WEBP},
	}
)

type PhotoProcessorWorker struct {
	redisClient *redis.Client
	db          db.PhotoRepository
	store       store.Store
	log         *log.Logger
	stopChan    chan struct{}
	doneChan    chan bool
}

func NewPhotoProcessorWorker(redisClient *redis.Client, cfg *config.Config, db db.PhotoRepository, s store.Store, l *log.Logger) *PhotoProcessorWorker {
	return &PhotoProcessorWorker{
		redisClient: redisClient,
		db:          db,
		store:       s,
		log:         l,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan bool, 1),
	}
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
	var processingJob PhotoProcessingJob
	if err := json.Unmarshal([]byte(msg.Payload), &processingJob); err != nil {
		return fmt.Errorf("error unmarshalling message payload %q: %w", msg.Payload, err)
	}

	var sizes []ImageOpts
	switch processingJob.Type {
	case JobTypeAlbumPhoto:
		sizes = AlbumPhotoSizes
	case JobTypeUserPhoto:
		sizes = ProfilePicSizes
	default:
		return fmt.Errorf("unknown photo type: %s", processingJob.Type)
	}

	if err := ppw.processPhoto(processingJob.PhotoID, sizes); err != nil {
		return fmt.Errorf("error processing photo ID %d: %w", processingJob.PhotoID, err)
	}
	return nil
}

func (ppw *PhotoProcessorWorker) updatePhotoStatus(photo domain.Photo, status domain.PhotoStatus) error {
	return ppw.db.UpdatePhotoStatus(context.Background(), photo.ID, status)
}

func (ppw *PhotoProcessorWorker) processPhoto(photoId int32, sizes []ImageOpts) error {
	ppw.log.Printf("processing photo ID %d", photoId)

	photo, err := ppw.db.GetPhoto(context.Background(), photoId)
	if err != nil {
		return fmt.Errorf("error getting photo from database: %v", err)
	}

	originalMeta, err := ppw.db.GetPhotoMetadataByPhotoIDAndVariant(context.Background(), photo.ID, domain.PhotoVariantOriginal)
	if err != nil {
		return fmt.Errorf("error getting original photo metadata: %v", err)
	}

	var processingErr bool
	defer func() {
		if processingErr {
			ppw.updatePhotoStatus(photo, domain.PhotoStatusErrored)
		}
	}()

	path, err := utils.BuildPhotoPathForVariant(photo.Key, domain.PhotoVariantOriginal, utils.MimeType(originalMeta.MimeType))
	if err != nil {
		processingErr = true
		return fmt.Errorf("error building photo path for original variant: %v", err)
	}
	photoReader, err := ppw.store.Read(context.Background(), path)
	if err != nil {
		processingErr = true
		return fmt.Errorf("error reading original photo from store: %v", err)
	}
	defer photoReader.Close()

	photoBytes, err := io.ReadAll(photoReader)
	if err != nil {
		processingErr = true
		return fmt.Errorf("error reading original photo data: %v", err)
	}

	meta, err := bimg.NewImage(photoBytes).Metadata()
	if err != nil {
		processingErr = true
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
			processingErr = true
			ppw.log.Printf("error processing %s image: %v", size.Variant, err)
			continue
		}

		imgSize, err := bimg.NewImage(processedImg).Size()
		if err != nil {
			processingErr = true
			ppw.log.Printf("error getting size of processed %s image: %v", size.Variant, err)
			continue
		}

		fileSize := int64(len(processedImg))
		photoMeta, err := ppw.db.CreatePhotoMetadata(context.Background(), domain.CreatePhotoMetadataParams{
			PhotoID:  photo.ID,
			Variant:  size.Variant,
			Width:    int32(imgSize.Width),
			Height:   int32(imgSize.Height),
			FileSize: &fileSize,
			MimeType: "image/webp",
		})
		if err != nil {
			processingErr = true
			ppw.log.Printf("error creating photo metadata for %q: %v", size.Variant, err)
			continue
		}

		variantPath, err := utils.BuildPhotoPathForVariant(photo.Key, photoMeta.Variant, utils.MimeTypeWEBP)
		if err != nil {
			processingErr = true
			ppw.log.Printf("error building photo path for %s variant: %v", size.Variant, err)
			continue
		}
		if err := ppw.store.Write(context.Background(), variantPath, bytes.NewReader(processedImg)); err != nil {
			processingErr = true
			ppw.log.Printf("error writing %s image to store: %v", size.Variant, err)
			continue
		}
	}

	if err := ppw.updatePhotoStatus(photo, domain.PhotoStatusProcessed); err != nil {
		processingErr = true
		ppw.log.Printf("error updating photo %d status: %v", photoId, err)
	}

	return nil
}

func (ppw *PhotoProcessorWorker) Stop() {
	close(ppw.stopChan)
	<-ppw.doneChan
}
