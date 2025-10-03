package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/pagination"
	"github.com/npezzotti/gophoto/store"
	"github.com/npezzotti/gophoto/workers"
)

const (
	FormFileName         = "file"
	MaxUploadSize        = 50 << (10 * 2)
	DefaultProfilePrefix = "images/profile"
	DefaultAlbumCover    = "images/album_cover.webp"
)

type UserImageResponse struct {
	Image        db.Photo
	URL          string
	OriginalURL  string
	ThumbnailURL string
	LargeURL     string
}

type AlbumResponse struct {
	Album         db.ListAlbumsByUserRow
	AlbumCoverUrl string
}

func (a *application) newAlbumResponse(ctx context.Context, album db.ListAlbumsByUserRow) *AlbumResponse {
	coverPhotos, err := a.database.ListPhotosByAlbum(ctx, db.ListPhotosByAlbumParams{
		AlbumID: album.ID,
		Offset:  0,
		Limit:   1,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.ErrorLog.Printf("error getting album cover: %s\n", err)
	}

	var coverUrl string
	if len(coverPhotos) > 0 {
		coverUrl, err = a.store.GenerateURL(ctx, coverPhotos[0].Key+string(store.FileSuffixThumbnail))
		if err != nil {
			a.ErrorLog.Printf("error generating url for %s: %s\n", coverPhotos[0].Key, err)
		}
	} else {
		coverUrl = filepath.Join("/assets", DefaultAlbumCover)
	}

	return &AlbumResponse{
		Album:         album,
		AlbumCoverUrl: coverUrl,
	}
}

func (a *application) newUserImageResponse(ctx context.Context, photo db.Photo) *UserImageResponse {
	original, err := a.store.GenerateURL(ctx, photo.Key+string(store.FileSuffixOriginal))
	if err != nil {
		a.ErrorLog.Printf("error generating url for photo %d: %s\n", photo.ID, err.Error())
	}

	thumbnail, err := a.store.GenerateURL(ctx, photo.Key+string(store.FileSuffixThumbnail))
	if err != nil {
		a.ErrorLog.Printf("error generating url for photo %d: %s\n", photo.ID, err.Error())
	}

	large, err := a.store.GenerateURL(ctx, photo.Key+string(store.FileSuffixLarge))
	if err != nil {
		a.ErrorLog.Printf("error generating url for photo %d: %s\n", photo.ID, err.Error())
	}

	return &UserImageResponse{
		Image:        photo,
		OriginalURL:  original,
		ThumbnailURL: thumbnail,
		LargeURL:     large,
	}
}

func (a *application) getAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.getUserFromRequest(r)
		id_str := r.URL.Query().Get("id")

		if id_str != "" {
			// Request for specific album
			id, err := strconv.Atoi(id_str)
			if err != nil {
				a.ErrorLog.Println("error converting string to int", err)
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			album, err := a.database.GetAlbum(r.Context(), int32(id))
			if err != nil {
				a.ErrorLog.Println("error getting album:", err)
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			if user.ID != album.UserID {
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
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
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			images := []*UserImageResponse{}
			for _, photo := range photos {
				imageResponse := a.newUserImageResponse(r.Context(), photo)
				images = append(images, imageResponse)
			}

			td := a.newTemplateData(r)
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
			a.ErrorLog.Printf("error listing albums: %s\n", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var albumResponse []*AlbumResponse
		for _, album := range albums {
			a := a.newAlbumResponse(r.Context(), album)
			albumResponse = append(albumResponse, a)
		}

		td := a.newTemplateData(r)
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
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		user := a.getUserFromRequest(r)
		album, err := a.database.CreateAlbum(r.Context(), db.CreateAlbumParams{
			UserID: user.ID,
			Title:  r.Form.Get("title"),
		})
		if err != nil {
			a.ErrorLog.Println("error creating album:", err)
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.Flash(r, fmt.Sprintf("Successfully created album \"%s\"!", album.Title), flashInfo)
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
				a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		album_id_str := r.URL.Query().Get("id")
		album_id, err := strconv.Atoi(album_id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int:", err)
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
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
			a.Flash(r, strings.ToLower(flashMsg), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		user := a.getUserFromRequest(r)
		if user.ID != album.UserID {
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
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
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.Flash(r, "Album updated!", flashInfo)
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
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
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
			a.Flash(r, strings.ToLower(flashMsg), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		user := a.getUserFromRequest(r)
		if user.ID != album.UserID {
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		if err := a.database.DeleteAlbum(r.Context(), int32(album.ID)); err != nil {
			a.ErrorLog.Println("error deleting album:", err)
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		a.Flash(r, fmt.Sprintf("Successfully deleted album %q.", album.Title), flashInfo)
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

	if !strings.HasPrefix(filetype, "image/") {
		resp := map[string]string{"error": "file type not allowed"}
		if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	key := uuid.New().String()
	photo, err := a.database.CreatePhoto(r.Context(), db.CreatePhotoParams{
		UserID: user.ID,
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

	if _, err = a.database.CreatePhotoMetadata(r.Context(), db.CreatePhotoMetadataParams{
		PhotoID:  photo.ID,
		Variant:  db.PhotoVariantOriginal,
		FileSize: sql.NullInt64{Int64: fh.Size, Valid: true},
	}); err != nil {
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

	if err := a.store.Write(r.Context(), photo.Key+string(store.FileSuffixOriginal), f); err != nil {
		a.ErrorLog.Printf("error writing photo to storage: %s\n", err.Error())
		resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
		if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
			a.ErrorLog.Println("error writing json response:", err)
		}
		return
	}

	// Process photo in background
	processingJob, err := json.Marshal(workers.PhotoProcessingJob{Type: workers.PhotoTypeUserPhoto, PhotoID: photo.ID})
	if err != nil {
		a.ErrorLog.Printf("error marshalling photo processing job: %s\n", err.Error())
		return
	}

	err = a.redisClient.Publish(context.Background(), workers.PhotoProcessingQueue, processingJob).Err()
	if err != nil {
		a.ErrorLog.Printf("error publishing photo processing job: %s\n", err.Error())
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

		// Ensure photo belongs to user
		if photo.UserID != a.getUserFromRequest(r).ID {
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
		a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error parsing id:", err)
		a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	photo, err := a.database.GetAlbumPhoto(r.Context(), int32(id))
	if err != nil {
		a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	user := a.getUserFromRequest(r)
	if photo.UserID != user.ID {
		a.Flash(r, strings.ToLower(http.StatusText(http.StatusNotFound)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// Set album_id to null to mark photo for deletion by storage cleanup worker
	if err := a.database.RemovePhotoFromAlbum(r.Context(), db.RemovePhotoFromAlbumParams{
		PhotoID: photo.ID,
		AlbumID: photo.AlbumID,
	}); err != nil {
		a.ErrorLog.Println("error removing photo from album:", err)
		a.Flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.Flash(r, "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", photo.AlbumID), http.StatusSeeOther)
}

func (a *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		lf := &LoginForm{
			Email:    r.Form.Get("email"),
			Password: r.Form.Get("password"),
		}

		if !lf.Validate() {
			td := a.newTemplateData(r)
			td.Form = lf
			if err := a.renderTemplate(w, td, "login.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}

		user, err := a.database.GetUserByEmail(r.Context(), lf.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				a.Flash(r, "No account found with that email address.", flashErr)
				td := a.newTemplateData(r)
				td.Form = lf

				w.WriteHeader(http.StatusForbidden)
				if err := a.renderTemplate(w, td, "login.html"); err != nil {
					a.ErrorLog.Printf("error rendering template: %s", err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			} else {
				a.ErrorLog.Printf("error getting user by email: %s", err)
				a.Flash(r, "Internal server error.", flashErr)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}

		if !passwordsMatch(user.PasswordHash, lf.Password) {
			a.Flash(r, "Incorrect password.", flashErr)

			td := a.newTemplateData(r)
			td.Form = lf

			if err := a.renderTemplate(w, td, "login.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := a.sessionManager.RenewToken(r.Context()); err != nil {
			a.ErrorLog.Printf("error renewing token: %s", err)
			a.Flash(r, "Internal server error.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		a.sessionManager.Put(r.Context(), SessionKeyUserID, user.ID)

		// Redirect to intended path after login
		path := a.sessionManager.PopString(r.Context(), SessionKeyRedirectPath)
		if path != "" {
			http.Redirect(w, r, path, http.StatusSeeOther)
			return
		}

		// Default redirect
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
	case http.MethodGet:
		td := a.newTemplateData(r)
		td.Form = &LoginForm{}
		if err := a.renderTemplate(w, td, "login.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) aboutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		td := a.newTemplateData(r)
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
