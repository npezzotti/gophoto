package workers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/config"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
	"github.com/redis/go-redis/v9"
)

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

func NewPhotoProcessorWorker(redisClient *redis.Client, cfg *config.Config, db *db.Queries, store store.Store, logger *log.Logger) (*PhotoProcessorWorker, error) {
	ppw := &PhotoProcessorWorker{
		redisClient: redisClient,
		db:          db,
		store:       store,
		log:         logger,
		imageOpts: []ImageOpts{
			{Suffix: "_thumb", Width: 300, Height: 300, Quality: 80, Type: bimg.WEBP},
			{Suffix: "_large", Width: 1920, Height: 1080, Quality: 90, Type: bimg.WEBP},
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
	subscriber := ppw.redisClient.Subscribe(context.Background(), PhotoProcessingQueue)

	go func() {
		for {
			select {
			case <-ppw.stopChan:
				ppw.log.Println("stopping photo processor worker")
				ppw.doneChan <- true
				return
			default:
				msg, err := subscriber.ReceiveMessage(context.Background())
				if err != nil {
					log.Print("error receiving message:", err)
					continue
				}
				log.Printf("received message: %s", msg.Payload)

				photoId, err := strconv.Atoi(msg.Payload)
				if err != nil {
					log.Printf("error parsing photo ID from message payload %q: %v", msg.Payload, err)
					continue
				}

				ppw.processPhoto(int32(photoId))
			}
		}
	}()
}

func (ppw *PhotoProcessorWorker) processPhoto(photoId int32) {
	ppw.log.Printf("starting photo processing job for photo ID %d", photoId)

	photo, err := ppw.db.GetPhoto(context.Background(), photoId)
	if err != nil {
		ppw.log.Printf("error getting photo from database: %v", err)
		return
	}

	photoBytes, err := ppw.downloadOriginal(photo)
	if err != nil {
		ppw.log.Printf("error downloading original photo: %v", err)
		return
	}

	meta, err := bimg.NewImage(photoBytes).Metadata()
	if err != nil {
		ppw.log.Printf("error getting image metadata: %v", err)
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
			ppw.log.Printf("error processing image for %s: %v", opts.Suffix, err)
			continue
		}

		if err := ppw.store.Write(context.Background(), photo.Key+opts.Suffix, bytes.NewReader(processedImg)); err != nil {
			ppw.log.Printf("error writing %s image to store: %v", opts.Suffix, err)
			continue
		}

		ppw.log.Printf("successfully processed and stored image %q", photo.Key+opts.Suffix)
	}

	if err := ppw.db.UpdatePhotoStatus(context.Background(), db.UpdatePhotoStatusParams{
		ID:     photo.ID,
		Status: "processed",
	}); err != nil {
		ppw.log.Printf("error updating photo status: %v", err)
	}

	ppw.log.Println("photo processing job completed")
}

func (ppw *PhotoProcessorWorker) downloadOriginal(photo db.Photo) ([]byte, error) {
	photoURL, err := ppw.store.Read(context.Background(), photo.Key+"_original")
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
