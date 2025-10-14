package web

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/npezzotti/gophoto/internal/workers"
	"github.com/npezzotti/gophoto/pkg/pagination"
)

const (
	FormFileName                = "file"
	MaxUploadSize               = 50 << (10 * 2)
	DefaultProfileThumbnailPath = "images/profile_thumb.webp"
	DefaultProfileAvatarPath    = "images/profile_avatar.webp"
	DefaultAlbumCover           = "images/album_cover.webp"
)

type AlbumResponse struct {
	Album           db.ListAlbumsByUserRow
	AlbumCoverImage ImageResponse
}

func (a *application) newAlbumResponse(ctx context.Context, album db.ListAlbumsByUserRow) *AlbumResponse {
	coverPhotos, err := a.database.ListPhotosByAlbum(ctx, db.ListPhotosByAlbumParams{
		AlbumID: album.ID,
		Offset:  0,
		Limit:   1,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.ErrorLog.Printf("error getting album cover: %s", err)
	}

	var imageResp ImageResponse
	if len(coverPhotos) > 0 {
		meta, err := a.database.GetPhotoMetadataByPhotoID(ctx, coverPhotos[0].ID)
		if err != nil {
			a.ErrorLog.Printf("error getting metadata for photo %d: %s", coverPhotos[0].ID, err)
		}

		var sources []Image
		var defaultSrc string
		for _, m := range meta {
			path, err := utils.BuildPhotoPath(coverPhotos[0].Key, m.Variant, utils.MimeType(m.MimeType))
			if err != nil {
				a.ErrorLog.Printf("error building path for %s: %s", coverPhotos[0].Key, err)
				continue
			}

			url, err := a.store.GenerateURL(ctx, path)
			if err != nil {
				a.ErrorLog.Printf("error generating url for %s: %s", coverPhotos[0].Key, err)
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
			Image:      coverPhotos[0],
			Alt:        coverPhotos[0].Key,
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

func (a *application) generateAlbumImageResponse(ctx context.Context, photo db.Photo) *ImageResponse {
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

	return &ImageResponse{
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
		user := a.getUserFromRequest(r)
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

			images := []*ImageResponse{}
			for _, photo := range photos {
				imageResponse := a.generateAlbumImageResponse(r.Context(), photo)
				images = append(images, imageResponse)
			}

			td := a.generateTemplateData(r)
			td.Album = album
			td.Images = images
			td.Paginator = pagination
			td.AddPhotoUploadAction = fmt.Sprintf("/photo/new?id=%d", album.ID)

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

		user := a.getUserFromRequest(r)
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

		user := a.getUserFromRequest(r)
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

		user := a.getUserFromRequest(r)
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

func (a *application) createPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	album_id_str := r.URL.Query().Get("id")
	album_id, err := strconv.Atoi(album_id_str)
	if err != nil {
		a.ErrorLog.Printf("error converting string to int: %s", err)
		resp := map[string]string{"error": "invalid album id"}
		if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	album, err := a.database.GetAlbum(r.Context(), int32(album_id))
	if err != nil {
		resp := map[string]string{"error": "album not found"}
		if err := a.writeJsonResp(w, http.StatusNotFound, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	user := a.getUserFromRequest(r)
	if user == nil {
		a.flash(r, "User not found.", flashErr)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.ID != album.UserID {
		resp := map[string]string{"error": "album not found"}
		if err := a.writeJsonResp(w, http.StatusNotFound, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	f, fh, err := r.FormFile(FormFileName)
	if err != nil {
		a.ErrorLog.Printf("error getting file from form: %s", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusBadRequest))}
		if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}
	defer f.Close()

	if fh.Size > MaxUploadSize {
		resp := map[string]string{"error": fmt.Sprintf("file size exceeds max upload size of %dMB", MaxUploadSize/1024/1024)}
		if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	filetype, err := detectContentType(f)
	if err != nil {
		a.ErrorLog.Println("error detecting content type:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	if !strings.HasPrefix(filetype, "image/") || !utils.ValidateMimeType(filetype) {
		resp := map[string]string{"error": "file type not allowed"}
		if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	key := uuid.New().String()
	photo, err := a.database.CreatePhoto(r.Context(), db.CreatePhotoParams{
		UserID: sql.NullInt32{Int32: user.ID, Valid: true},
		Key:    key,
		Status: db.PhotoStatusProcessing,
	})
	if err != nil {
		a.ErrorLog.Println("error creating photo:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		a.ErrorLog.Println("error reading uploaded file:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	meta, err := bimg.NewImage(buf).Size()
	if err != nil {
		a.ErrorLog.Println("error getting image size:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	photoMeta, err := a.database.CreatePhotoMetadata(r.Context(), db.CreatePhotoMetadataParams{
		PhotoID:  photo.ID,
		Variant:  db.PhotoVariantOriginal,
		Width:    int32(meta.Width),
		Height:   int32(meta.Height),
		FileSize: sql.NullInt64{Int64: fh.Size, Valid: true},
		MimeType: filetype,
	})
	if err != nil {
		a.ErrorLog.Println("error creating photo metadata:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	if err := a.database.AddPhotoToAlbum(r.Context(), db.AddPhotoToAlbumParams{
		AlbumID: album.ID,
		PhotoID: photo.ID,
	}); err != nil {
		a.ErrorLog.Println("error adding photo to album:", err)
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	path, err := utils.BuildPhotoPath(photo.Key, photoMeta.Variant, utils.MimeType(filetype))
	if err != nil {
		a.ErrorLog.Printf("error building photo path: %s", err.Error())
	}
	if err := a.store.Write(r.Context(), path, bytes.NewReader(buf)); err != nil {
		a.ErrorLog.Printf("error writing photo to storage: %s", err.Error())
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	// Process photo in background
	processingJob, err := json.Marshal(workers.PhotoProcessingJob{Type: workers.JobTypeAlbumPhoto, PhotoID: photo.ID})
	if err != nil {
		a.ErrorLog.Printf("error marshalling photo processing job: %s", err.Error())
		return
	}

	err = a.redisClient.Publish(context.Background(), workers.PhotoProcessingQueue, processingJob).Err()
	if err != nil {
		a.ErrorLog.Printf("error publishing photo processing job: %s", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int32{"id": photo.ID}); err != nil {
		a.ErrorLog.Println("error encoding json:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (a *application) photoStatusHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id_str := r.URL.Query().Get("id")
		if id_str == "" {
			if err := a.writeJsonResp(w, http.StatusBadRequest, map[string]string{"error": strings.ToLower(http.StatusText(http.StatusBadRequest))}); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}

		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			if err := a.writeJsonResp(w, http.StatusBadRequest, map[string]string{"error": strings.ToLower(http.StatusText(http.StatusBadRequest))}); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		photo, err := a.database.GetPhoto(r.Context(), int32(id))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if err := a.writeJsonResp(w, http.StatusNotFound, map[string]string{"error": "photo not found"}); err != nil {
					a.ErrorLog.Println("error writing json response:", err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
				return
			}
			a.ErrorLog.Println("error getting photo:", err)
			if err := a.writeJsonResp(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"}); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		user := a.getUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if !photo.UserID.Valid || photo.UserID.Int32 != user.ID {
			a.writeJsonResp(w, http.StatusNotFound, map[string]string{"error": "photo not found"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := a.writeJsonResp(w, http.StatusOK, map[string]string{"status": string(photo.Status)}); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
			return
		}
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

	user := a.getUserFromRequest(r)
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

// writeJsonResp writes the provided data as a JSON response with the specified HTTP status code.
func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}
