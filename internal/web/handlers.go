package web

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/pagination"
)

const (
	FormFileName                = "file"
	MaxUploadSize               = 50 << (10 * 2) // 50 MB
	DefaultProfileThumbnailPath = "images/profile_thumb.webp"
	DefaultProfileAvatarPath    = "images/profile_avatar.webp"
	DefaultAlbumCover           = "images/album_cover.webp"
)

type AlbumResponse struct {
	Album           db.ListAlbumsByUserRow
	AlbumCoverImage ImageResponse
}

func (a *application) newAlbumResponse(ctx context.Context, album db.ListAlbumsByUserRow) *AlbumResponse {
	var imageResp ImageResponse
	if album.CoverPhotoID.Valid {
		meta, err := a.database.GetPhotoMetadataByPhotoID(ctx, album.CoverPhotoID.Int32)
		if err != nil {
			a.ErrorLog.Printf("error getting metadata for photo %d: %s", album.CoverPhotoID.Int32, err)
		}

		var sources []Image
		var defaultSrc string
		for _, m := range meta {
			path, err := utils.BuildPhotoPath(album.CoverPhotoKey.String, m.Variant, utils.MimeType(m.MimeType))
			if err != nil {
				a.ErrorLog.Printf("error building path for %s: %s", album.CoverPhotoKey.String, err)
				continue
			}

			url, err := a.store.GenerateURL(ctx, path)
			if err != nil {
				a.ErrorLog.Printf("error generating url for photo %d with variant %s: %s", album.CoverPhotoID.Int32, m.Variant, err)
				continue
			}

			sources = append(sources, Image{
				Width:  m.Width,
				Height: m.Height,
				URL:    url,
			})

			if m.Variant == db.PhotoVariantLarge {
				defaultSrc = url
			}
		}

		imageResp = ImageResponse{
			Image:      db.Photo{ID: album.CoverPhotoID.Int32, Key: album.CoverPhotoKey.String},
			Alt:        album.CoverPhotoKey.String,
			DefaultSrc: defaultSrc,
			Sources:    sources,
		}
	} else {
		imageResp = ImageResponse{
			DefaultSrc: filepath.Join(a.config.StaticDir, DefaultAlbumCover),
			Sources: []Image{
				{Width: 400, Height: 300, URL: filepath.Join(a.config.StaticDir, DefaultAlbumCover)},
			},
			Alt: "Default album cover",
		}
	}

	return &AlbumResponse{
		Album:           album,
		AlbumCoverImage: imageResp,
	}
}

func (a *application) generateAlbumImageResponse(ctx context.Context, photo db.Photo) ImageResponse {
	photoMeta, err := a.database.GetPhotoMetadataByPhotoID(ctx, photo.ID)
	if err != nil {
		a.ErrorLog.Printf("error getting metadata for photo %d: %s", photo.ID, err.Error())
	}

	var sources []Image
	var originalUrl, defaultUrl string
	for _, meta := range photoMeta {
		path, err := utils.BuildPhotoPath(photo.Key, meta.Variant, utils.MimeType(meta.MimeType))
		if err != nil {
			a.ErrorLog.Printf("error building path for photo %d variant %s: %s", photo.ID, meta.Variant, err.Error())
		}

		url, err := a.store.GenerateURL(ctx, path)
		if err != nil {
			a.ErrorLog.Printf("error generating url for photo %d: %s", photo.ID, err.Error())
		}

		if meta.Variant != db.PhotoVariantOriginal {
			sources = append(sources, Image{
				Width:  meta.Width,
				Height: meta.Height,
				URL:    url,
			})
		}

		switch meta.Variant {
		case db.PhotoVariantOriginal:
			originalUrl = url
		case db.PhotoVariantLarge:
			defaultUrl = url
		default:
		}
	}

	return ImageResponse{
		Image:       photo,
		Alt:         photo.Key,
		OriginalSrc: originalUrl,
		DefaultSrc:  defaultUrl,
		Sources:     sources,
	}
}

func (a *application) getAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if id_str := r.URL.Query().Get("id"); id_str != "" {
			// Request for specific album
			id, err := strconv.Atoi(id_str)
			if err != nil {
				a.ErrorLog.Println("error converting string to int", err)
				a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			album, err := a.database.GetAlbum(r.Context(), int32(id))
			if err != nil {
				a.ErrorLog.Println("error getting album:", err)
				a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			if user.ID != album.UserID {
				a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			pagination := pagination.NewPaginationFromRequest(r, int(album.NumPhotos))
			photos, err := a.database.ListPhotosByAlbum(r.Context(), db.ListPhotosByAlbumParams{
				AlbumID: album.ID,
				Limit:   int32(pagination.Limit),
				Offset:  int32(pagination.Offset()),
			})
			if err != nil {
				a.flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			images := []ImageResponse{}
			for _, photo := range photos {
				imageResponse := a.generateAlbumImageResponse(r.Context(), photo)
				images = append(images, imageResponse)
			}

			td := a.generateTemplateData(r)
			td.Album = album
			td.Images = images
			td.Paginator = pagination
			td.AddPhotoUploadAction = fmt.Sprintf("/api/photos?type=album&id=%d", album.ID)

			if err := a.renderTemplate(w, td, "album.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			return
		}

		// Request for all albums
		totalAlbums, err := a.database.CountAlbumsByUser(r.Context(), user.ID)
		if err != nil {
			a.ErrorLog.Println("error counting albums:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pagination := pagination.NewPaginationFromRequest(r, int(totalAlbums))

		albums, err := a.database.ListAlbumsByUser(r.Context(), db.ListAlbumsByUserParams{
			UserID: user.ID,
			Limit:  int32(pagination.Limit),
			Offset: int32(pagination.Offset()),
		})
		if err != nil {
			a.ErrorLog.Printf("error listing albums: %s", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var albumResponse []*AlbumResponse
		for _, album := range albums {
			a := a.newAlbumResponse(r.Context(), album)
			albumResponse = append(albumResponse, a)
		}

		td := a.generateTemplateData(r)
		td.Albums = albumResponse
		td.Paginator = pagination

		if err := a.renderTemplate(w, td, "albums.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (a *application) createAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			if err := r.ParseForm(); err != nil {
				a.ErrorLog.Println("error parsing form:", err)
				a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		album, err := a.database.CreateAlbum(r.Context(), db.CreateAlbumParams{
			UserID: user.ID,
			Title:  r.Form.Get("title"),
		})
		if err != nil {
			a.ErrorLog.Println("error creating album:", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.flash(r, fmt.Sprintf("Successfully created album \"%s\"!", album.Title), flashInfo)
		http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", album.ID), http.StatusSeeOther)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) updateAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			if err := r.ParseForm(); err != nil {
				a.ErrorLog.Println("error parsing form:", err)
				a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		album_id_str := r.URL.Query().Get("id")
		album_id, err := strconv.Atoi(album_id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int:", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		album, err := a.database.GetAlbum(r.Context(), int32(album_id))
		if err != nil {
			var flashMsg string
			if errors.Is(err, sql.ErrNoRows) {
				flashMsg = "Album not found"
			} else {
				flashMsg = "Error retrieving album"
			}
			a.flash(r, strings.ToLower(flashMsg), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if user.ID != album.UserID {
			a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		album_params := db.UpdateAlbumParams{
			ID:        album.ID,
			UserID:    album.UserID,
			Title:     r.Form.Get("title"),
			UpdatedAt: time.Now(),
		}
		if err := a.database.UpdateAlbum(r.Context(), album_params); err != nil {
			a.ErrorLog.Println("error updating album:", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.flash(r, "Album successfully updated.", flashInfo)
		http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", album.ID), http.StatusSeeOther)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) deleteAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id_str := r.URL.Query().Get("id")
		if id_str == "" {
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		album, err := a.database.GetAlbum(r.Context(), int32(id))
		if err != nil {
			var flashMsg string
			if errors.Is(err, sql.ErrNoRows) {
				flashMsg = "Album not found"
			} else {
				flashMsg = "Error retrieving album"
			}
			a.flash(r, strings.ToLower(flashMsg), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if user.ID != album.UserID {
			a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		if err := a.database.DeleteAlbum(r.Context(), int32(album.ID)); err != nil {
			a.ErrorLog.Println("error deleting album:", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.flash(r, fmt.Sprintf("Successfully deleted album %q.", album.Title), flashInfo)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
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

func (a *application) uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user, err := a.authenticateRequest(r)
	if err != nil {
		a.ErrorLog.Printf("authentication error: %v", err)
		a.writeJsonErrorResp(w, http.StatusUnauthorized, strings.ToLower(http.StatusText(http.StatusUnauthorized)))
		return
	}

	photoType := r.URL.Query().Get("type")
	if photoType == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "missing photo type")
		return
	}

	buf, fileType, meta, err := a.processUploadedFile(r)
	if err != nil {
		a.ErrorLog.Println("error processing uploaded file:", err)
		a.writeJsonErrorResp(w, http.StatusBadRequest, strings.ToLower(http.StatusText(http.StatusBadRequest)))
		return
	}

	key := uuid.New().String()
	var photo db.Photo
	switch photoType {
	case "album":
		album_id_str := r.URL.Query().Get("id")
		album_id, err := strconv.Atoi(album_id_str)
		if err != nil {
			a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid album id")
			return
		}

		album, err := a.database.GetAlbum(r.Context(), int32(album_id))
		if err != nil {
			var respMsg string
			if errors.Is(err, sql.ErrNoRows) {
				respMsg = strings.ToLower(http.StatusText(http.StatusNotFound))
			} else {
				a.ErrorLog.Println("error retrieving album:", err)
				respMsg = strings.ToLower(http.StatusText(http.StatusInternalServerError))
			}
			a.writeJsonErrorResp(w, http.StatusBadRequest, respMsg)
			return
		}

		if user.ID != album.UserID {
			a.writeJsonErrorResp(w, http.StatusNotFound, strings.ToLower(http.StatusText(http.StatusNotFound)))
			return
		}

		photo, err = a.database.CreateAlbumPhotoWithOriginalMetadata(r.Context(), album.ID, db.CreatePhotoWithOriginalMetadataParams{
			UserID:   sql.NullInt32{Int32: user.ID, Valid: true},
			Key:      key,
			Width:    int32(meta.Width),
			Height:   int32(meta.Height),
			FileSize: sql.NullInt64{Int64: int64(len(buf)), Valid: true},
			MimeType: string(fileType),
		})
		if err != nil {
			a.ErrorLog.Printf("error creating photo: %s", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, strings.ToLower(http.StatusText(http.StatusInternalServerError)))
			return
		}
	case "user":
		photo, err = a.database.CreateUserPhotoWithOriginalMetadata(r.Context(), user, db.CreatePhotoWithOriginalMetadataParams{
			UserID:   sql.NullInt32{Int32: user.ID, Valid: true},
			Key:      key,
			Width:    int32(meta.Width),
			Height:   int32(meta.Height),
			FileSize: sql.NullInt64{Int64: int64(len(buf)), Valid: true},
			MimeType: string(fileType),
		})
		if err != nil {
			a.ErrorLog.Printf("error creating photo: %s", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, strings.ToLower(http.StatusText(http.StatusInternalServerError)))
			return
		}
	default:
		a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid photo type")
		return
	}

	if err := a.uploadPhotoToStorage(r.Context(), photo, buf, fileType); err != nil {
		a.ErrorLog.Printf("error uploading photo %d to storage: %s", photo.ID, err)
		resp := map[string]string{"error": http.StatusText(http.StatusInternalServerError)}
		a.writeJsonResp(w, http.StatusInternalServerError, resp)
		return
	}

	if err := a.queuePhotoProcessing(r.Context(), photo); err != nil {
		// Log the error but do not fail the request
		a.ErrorLog.Printf("error queueing photo %d for processing: %s", photo.ID, err)
	}

	a.writeJsonResp(w, http.StatusOK, map[string]int32{"id": photo.ID})
}

func (a *application) photoStatusHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user, err := a.authenticateRequest(r)
		if err != nil {
			a.writeJsonErrorResp(w, http.StatusUnauthorized, strings.ToLower(http.StatusText(http.StatusUnauthorized)))
			return
		}

		id_str := r.URL.Query().Get("id")
		if id_str == "" {
			a.writeJsonErrorResp(w, http.StatusBadRequest, strings.ToLower(http.StatusText(http.StatusBadRequest)))
			return
		}

		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			a.writeJsonErrorResp(w, http.StatusBadRequest, strings.ToLower(http.StatusText(http.StatusBadRequest)))
			return
		}

		photo, err := a.database.GetPhoto(r.Context(), int32(id))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				a.writeJsonErrorResp(w, http.StatusNotFound, strings.ToLower(http.StatusText(http.StatusNotFound)))
				return
			}
			a.ErrorLog.Println("error getting photo:", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, strings.ToLower(http.StatusText(http.StatusInternalServerError)))
			return
		}

		if !photo.UserID.Valid || photo.UserID.Int32 != user.ID {
			a.writeJsonErrorResp(w, http.StatusNotFound, strings.ToLower(http.StatusText(http.StatusNotFound)))
			return
		}

		a.writeJsonResp(w, http.StatusOK, map[string]string{"status": string(photo.Status)})
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) deletePhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error parsing id:", err)
		a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	photo, err := a.database.GetAlbumPhoto(r.Context(), int32(id))
	if err != nil {
		a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	user := a.extractUserFromRequest(r)
	if user == nil {
		a.flash(r, "User not found.", flashErr)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Ensure photo belongs to user
	if !photo.UserID.Valid || photo.UserID.Int32 != user.ID {
		a.flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// Set album_id to null to mark photo for deletion by storage cleanup worker
	if err := a.database.RemovePhotoFromAlbum(r.Context(), db.RemovePhotoFromAlbumParams{
		PhotoID: photo.ID,
		AlbumID: photo.AlbumID,
	}); err != nil {
		a.ErrorLog.Println("error removing photo from album:", err)
		a.flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.flash(r, "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", photo.AlbumID), http.StatusSeeOther)
}

func (a *application) aboutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		td := a.generateTemplateData(r)
		if err := a.renderTemplate(w, td, "about.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
