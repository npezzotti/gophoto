package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/pagination"
	"github.com/npezzotti/gophoto/store"
	"github.com/npezzotti/gophoto/workers"
)

const (
	formFileName      = "file"
	maxUploadSize     = 50 << (10 * 2)
	defaultProfilePic = "images/profile.png"
	defaultAlbumCover = "images/album.png"
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
	coverPhoto, err := a.database.ListPhotosByAlbum(ctx, db.ListPhotosByAlbumParams{
		AlbumID: sql.NullInt32{Int32: album.ID, Valid: true},
		Offset:  0,
		Limit:   1,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.ErrorLog.Printf("error getting album cover: %s\n", err)
	}

	coverUrl := defaultAlbumCover

	if len(coverPhoto) > 0 {
		coverUrl, err = a.store.Read(ctx, coverPhoto[0].Key+string(store.FileSuffixThumbnail))
		if err != nil {
			a.ErrorLog.Printf("error generating url for %s: %s\n", coverPhoto[0].Key, err)
		}
	}

	return &AlbumResponse{
		Album:         album,
		AlbumCoverUrl: coverUrl,
	}
}

func (a *application) newUserImageResponse(ctx context.Context, photo db.Photo) *UserImageResponse {
	original, err := a.store.Read(ctx, photo.Key+"_original")
	if err != nil {
		a.ErrorLog.Printf("error generating url for photo %d: %s\n", photo.ID, err.Error())
	}

	thumbnail, err := a.store.Read(ctx, photo.Key+"_thumb")
	if err != nil {
		a.ErrorLog.Printf("error generating url for photo %d: %s\n", photo.ID, err.Error())
	}

	large, err := a.store.Read(ctx, photo.Key+string(store.FileSuffixLarge))
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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user := a.getUserFromRequest(r)

	id_str := r.URL.Query().Get("id")

	if id_str != "" {
		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		album, err := a.database.GetAlbum(r.Context(), int32(id))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if user.ID != album.UserID {
			http.NotFound(w, r)
			return
		}

		pagination := pagination.NewPaginationFromRequest(r, int(album.NumPhotos))

		photos, err := a.database.ListPhotosByAlbum(r.Context(), db.ListPhotosByAlbumParams{
			AlbumID: sql.NullInt32{Int32: album.ID, Valid: true},
			Limit:   int32(pagination.Limit),
			Offset:  int32(pagination.Offset()),
		})
		if err != nil {
			http.NotFound(w, r)
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

		if err := a.renderTemplate(w, td, "album.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		return
	}

	totalAlbums, err := a.database.CountAlbumsByUser(r.Context(), user.ID)
	if err != nil {
		a.ErrorLog.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
}

func (a *application) createAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user := a.getUserFromRequest(r)

	if err := r.ParseForm(); err != nil {
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	album, err := a.database.CreateAlbum(r.Context(), db.CreateAlbumParams{
		UserID: user.ID,
		Title:  r.Form.Get("title"),
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	a.Flash(r, fmt.Sprintf("Successfully created album \"%s\"!", album.Title), flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", album.ID), http.StatusSeeOther)
}

func (a *application) updateAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	album_id_str := r.URL.Query().Get("id")
	album_id, err := strconv.Atoi(album_id_str)
	if err != nil {
		a.ErrorLog.Println("error converting string to int:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	album, err := a.database.GetAlbum(r.Context(), int32(album_id))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := a.getUserFromRequest(r)

	if user.ID != album.UserID {
		http.NotFound(w, r)
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	redirectUrl := fmt.Sprintf("/albums?id=%d", album.ID)

	a.Flash(r, "Album updated!", flashInfo)
	http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
}

func (a *application) deleteAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	id_str := r.URL.Query().Get("id")

	if id_str != "" {
		id, err := strconv.Atoi(id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		album, err := a.database.GetAlbum(r.Context(), int32(id))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		user := a.getUserFromRequest(r)

		if user.ID != album.UserID {
			http.NotFound(w, r)
			return
		}

		if err := a.database.DeleteAlbum(r.Context(), int32(album.ID)); err != nil {
			a.ErrorLog.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		a.Flash(r, "Album deleted!", flashInfo)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	album, err := a.database.GetAlbum(r.Context(), int32(album_id))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := a.getUserFromRequest(r)
	if user.ID != album.UserID {
		http.NotFound(w, r)
		return
	}

	file, fh, err := r.FormFile(formFileName)
	if err != nil {
		a.ErrorLog.Printf("error getting file from form: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if fh.Size > maxUploadSize {
		a.Flash(r, "file too large", flashErr)
		http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", album_id), http.StatusSeeOther)
		return
	}

	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		a.ErrorLog.Println("error reading file:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	filetype := http.DetectContentType(buff)
	if filetype != "image/jpeg" && filetype != "image/png" {
		a.Flash(r, "Image must be PNG or JPEG", flashErr)

		http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", album_id), http.StatusSeeOther)
		return
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		a.ErrorLog.Printf("seek: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	key := uuid.New().String()
	photo, err := a.database.CreatePhoto(r.Context(), db.CreatePhotoParams{
		AlbumID: sql.NullInt32{Int32: int32(album_id), Valid: true},
		UserID:  user.ID,
		Key:     key,
		Status:  db.PhotoStatusProcessing,
	})
	if err != nil {
		a.ErrorLog.Println("error creating photo:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := a.store.Write(r.Context(), photo.Key+"_original", file); err != nil {
		a.ErrorLog.Printf("error writing photo to storage: %s\n", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Process photo in background
	processingJob, err := json.Marshal(workers.PhotoProcessingJob{PhotoID: photo.ID})
	if err != nil {
		a.ErrorLog.Printf("error marshalling photo processing job: %s\n", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = a.redisClient.Publish(context.Background(), workers.PhotoProcessingQueue, processingJob).Err()
	if err != nil {
		a.ErrorLog.Printf("error publishing photo processing job: %s\n", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error converting string to int", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	photo, err := a.database.GetPhoto(r.Context(), int32(id))
	if err != nil {
		a.ErrorLog.Println("photo file not found", err)
		http.NotFound(w, r)
		return
	}

	// Todo: check user owns photo

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]string{"status": string(photo.Status)}); err != nil {
		a.ErrorLog.Println("error encoding json:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
		a.Flash(r, "missing id parameter", flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error parsing id:", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	photo, err := a.database.GetPhoto(r.Context(), int32(id))
	if err != nil {
		a.Flash(r, "Photo not found", flashErr)
		http.NotFound(w, r)
		return
	}

	user := a.getUserFromRequest(r)
	if photo.UserID != user.ID {
		http.NotFound(w, r)
		return
	}

	updateParams := db.UpdatePhotoParams{
		ID:        photo.ID,
		AlbumID:   sql.NullInt32{Int32: photo.AlbumID.Int32, Valid: false}, // disassociate photo from album to mark for deletion
		Key:       photo.Key,
		Status:    photo.Status,
		UpdatedAt: time.Now(),
	}
	if _, err := a.database.UpdatePhoto(r.Context(), updateParams); err != nil {
		a.ErrorLog.Println("error updating photo:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	a.Flash(r, "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", photo.AlbumID.Int32), http.StatusSeeOther)
}

func (a *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		lf := &LoginForm{
			Email:    r.Form.Get("email"),
			Password: r.Form.Get("password"),
		}

		if !lf.Validate() {
			td := a.newTemplateData(r)
			td.Form = lf

			w.WriteHeader(http.StatusForbidden)
			if err := a.renderTemplate(w, td, "login.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			return
		}

		user, err := a.database.GetUserByEmail(r.Context(), lf.Email)
		if err != nil {
			var flashMsg string
			if errors.Is(err, sql.ErrNoRows) {
				flashMsg = "No account found with that email address."
				a.Flash(r, flashMsg, flashErr)

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
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		if !passwordsMatch(user.PasswordHash, lf.Password) {
			a.Flash(r, "Incorrect password.", flashErr)

			td := a.newTemplateData(r)
			td.Form = lf

			w.WriteHeader(http.StatusForbidden)
			if err := a.renderTemplate(w, td, "login.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			return
		}

		if err := a.sessionManager.RenewToken(r.Context()); err != nil {
			a.ErrorLog.Printf("error renewing token: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		a.sessionManager.Put(r.Context(), SessionKeyUserID, user.ID)

		path := a.sessionManager.PopString(r.Context(), SessionKeyRedirectPath)
		if path != "" {
			http.Redirect(w, r, path, http.StatusSeeOther)
			return
		}

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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	td := a.newTemplateData(r)

	if err := a.renderTemplate(w, td, "about.html"); err != nil {
		a.ErrorLog.Printf("error rendering template: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
