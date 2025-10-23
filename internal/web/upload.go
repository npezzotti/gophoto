package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
)

func (a *application) processUploadedFile(r *http.Request) ([]byte, utils.MimeType, bimg.ImageSize, error) {
	if err := r.ParseForm(); err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error parsing form: %w", err)
	}

	f, fh, err := r.FormFile(FormFileName)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error retrieving form file: %w", err)
	}
	defer f.Close()

	fileType, err := detectContentType(f)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error detecting content type: %w", err)
	}

	if err := validatePhotoUpload(fileType, fh); err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error validating photo upload: %w", err)
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error reading uploaded file: %w", err)
	}

	meta, err := bimg.NewImage(buf).Size()
	if err != nil {
		return nil, "", bimg.ImageSize{}, fmt.Errorf("error getting image size: %w", err)
	}

	return buf, utils.MimeType(fileType), meta, nil
}

func (a *application) uploadPhotoToStorage(ctx context.Context, photo db.Photo, buf []byte, fileType utils.MimeType) error {
	path, err := utils.BuildPhotoPath(photo.Key, db.PhotoVariantOriginal, fileType)
	if err != nil {
		return fmt.Errorf("error building photo path: %w", err)
	}
	if err := a.store.Write(ctx, path, bytes.NewReader(buf)); err != nil {
		return fmt.Errorf("error writing photo to storage: %w", err)
	}
	return nil
}

func validatePhotoUpload(fileType string, fh *multipart.FileHeader) error {
	if fh.Size > MaxUploadSize {
		return fmt.Errorf("file size exceeds max upload size of %dMB", MaxUploadSize/1024/1024)
	}

	if !strings.HasPrefix(fileType, "image/") || !utils.ValidateMimeType(fileType) {
		return fmt.Errorf("file type not allowed")
	}
	return nil
}

// detectContentType reads the first 512 bytes of the provided file to determine its content type.
// It resets the file's read pointer to the beginning before returning.
func detectContentType(f multipart.File) (string, error) {
	buff := make([]byte, 512)
	_, err := f.Read(buff)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	filetype := http.DetectContentType(buff)

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("seek: %s", err)
	}
	return filetype, nil
}
