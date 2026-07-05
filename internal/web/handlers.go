package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/pagination"
)

const (
	FormFileName         = "file"
	maxUploadRequestSize = 50 << (10 * 2) // 50 MB
	multipartMemoryLimit = 8 << 20
)

func (a *application) getAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if id_str := r.URL.Query().Get("id"); id_str != "" {
			// Request for specific album
			id, err := strconv.Atoi(id_str)
			if err != nil {
				a.Logger.Error("error converting string to int: %v", err)
				a.flash(r.Context(), "Invalid album ID.", flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			paginator := pagination.NewPaginationFromRequest(r)

			albumPageView, err := a.albumService.GetAlbumPageView(r.Context(), user.ID, int32(id), int32(paginator.Limit), int32(paginator.Offset()))
			if err != nil {
				var errMsg string
				if errors.Is(err, domain.ErrAlbumNotFound) {
					errMsg = "Album not found."
				} else {
					errMsg = "Internal server error."
				}
				a.Logger.Error("error getting album page view: %v", err)
				a.flash(r.Context(), errMsg, flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			paginator.SetTotal(int(albumPageView.TotalPhotos))
			td := a.generateTemplateData(r)
			td.Album = albumPageView.Album
			td.Images = albumPageView.Photos
			td.Paginator = paginator
			td.AddPhotoUploadAction = fmt.Sprintf("/photos?type=album&id=%d", albumPageView.Album.ID)

			a.renderTemplate(w, td, "album.html")
			return
		}

		pagination := pagination.NewPaginationFromRequest(r)

		albums, err := a.albumService.ListAlbumsByUser(r.Context(), user.ID, int32(pagination.Limit), int32(pagination.Offset()))
		if err != nil {
			a.Logger.Error("error listing albums: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pagination.SetTotal(len(albums))
		td := a.generateTemplateData(r)
		td.Albums = albums
		td.Paginator = pagination

		a.renderTemplate(w, td, "albums.html")
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (a *application) createAlbumHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.Logger.Error("error parsing form: %v", err)
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		title := r.Form.Get("title")
		if title == "" {
			a.flash(r.Context(), "Album title cannot be empty.", flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}

		album, err := a.albumService.CreateAlbum(r.Context(), user.ID, title)
		if err != nil {
			a.Logger.Error("error creating album: %v", err)
			a.flash(r.Context(), "Error creating album.", flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}

		a.flash(r.Context(), fmt.Sprintf("Successfully created album \"%s\"!", album.Title), flashInfo)
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
			a.Logger.Error("error parsing form: %v", err)
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}

		albumIDStr := r.URL.Query().Get("id")
		albumID, err := strconv.Atoi(albumIDStr)
		if err != nil {
			a.Logger.Error("error converting string to int: %v", err)
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}

		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if _, err := a.albumService.UpdateAlbum(r.Context(), int32(albumID), user.ID, r.Form.Get("title")); err != nil {
			if errors.Is(err, domain.ErrAlbumNotFound) {
				a.flash(r.Context(), "Album not found.", flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}
			a.Logger.Error("error updating album: %v", err)
			a.flash(r.Context(), "Error updating album.", flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}

		a.flash(r.Context(), "Album successfully updated.", flashInfo)
		http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", albumID), http.StatusSeeOther)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) deleteAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		a.Logger.Error("error parsing form: %v", err)
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.flash(r.Context(), "User not found.", flashErr)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	albumIDStr := r.Form.Get("id")
	if albumIDStr == "" {
		a.flash(r.Context(), "Album ID is required.", flashErr)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
		return
	}
	albumID, err := strconv.Atoi(albumIDStr)
	if err != nil {
		a.Logger.Error("error converting string to int: %v", err)
		a.flash(r.Context(), "Invalid album ID.", flashErr)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
		return
	}

	if err := a.albumService.DeleteAlbum(r.Context(), int32(albumID), user.ID); err != nil {
		if errors.Is(err, domain.ErrAlbumNotFound) {
			a.flash(r.Context(), "Album not found.", flashErr)
			http.Redirect(w, r, "/albums", http.StatusSeeOther)
			return
		}
		a.Logger.Error("error deleting album: %v", err)
		a.flash(r.Context(), "Error deleting album.", flashErr)
		http.Redirect(w, r, "/albums", http.StatusSeeOther)
		return
	}

	a.flash(r.Context(), fmt.Sprintf("Successfully deleted album with ID %d.", albumID), flashInfo)
	http.Redirect(w, r, "/albums", http.StatusSeeOther)
}

func (a *application) uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !isAuthenticated(r) {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, "user not found.")
		return
	}

	photoType := PhotoType(strings.ToLower(r.URL.Query().Get("type")))
	if photoType == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "missing \"type\" query parameter")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestSize)
	if err := r.ParseMultipartForm(multipartMemoryLimit); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			a.writeJsonErrorResp(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}

		a.writeJsonErrorResp(w, http.StatusBadRequest, "error parsing multipart form")
		return
	}

	f, fh, err := r.FormFile(FormFileName)
	if err != nil {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "error retrieving form file")
		return
	}
	defer f.Close()

	var photo domain.Photo
	switch photoType {
	case PhotoTypeAlbumPhoto:
		albumIDStr := r.URL.Query().Get("id")
		albumID, err := strconv.Atoi(albumIDStr)
		if err != nil {
			a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid album id")
			return
		}

		photo, err = a.photoService.CreateAlbumPhotoWithOriginalMetadata(r.Context(), f, fh, user.ID, int32(albumID))
		if err != nil {
			a.Logger.Error("error creating photo: %v", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	case PhotoTypeUserPhoto:
		photo, err = a.photoService.CreateUserPhotoWithOriginalMetadata(r.Context(), f, fh, user.ID)
		if err != nil {
			a.Logger.Error("error creating photo: %v", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	default:
		a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid photo type")
		return
	}

	a.writeJsonResp(w, http.StatusCreated, map[string]int32{"id": photo.ID})
}

func (a *application) photoStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !isAuthenticated(r) {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, "User not found.")
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "missing \"id\" query parameter")
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.Logger.Error("error converting string to int: %v", err)
		a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid \"id\" query parameter")
		return
	}

	photo, err := a.photoService.GetPhoto(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, domain.ErrPhotoNotFound) {
			a.writeJsonErrorResp(w, http.StatusNotFound, "photo not found")
			return
		}
		a.Logger.Error("error getting photo: %v", err)
		a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	if photo.UserID == nil || *photo.UserID != user.ID {
		a.writeJsonErrorResp(w, http.StatusNotFound, "photo not found")
		return
	}

	a.writeJsonResp(w, http.StatusOK, map[string]string{"status": string(photo.Status)})
}

func (a *application) deleteAlbumPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		a.Logger.Error("error parsing form: %v", err)
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	albumPhotoIDStr := r.Form.Get("id")
	if albumPhotoIDStr == "" {
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	albumPhotoID, err := strconv.Atoi(albumPhotoIDStr)
	if err != nil {
		a.Logger.Error("error parsing id: %v", err)
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.flash(r.Context(), "User not found.", flashErr)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := a.photoService.RemovePhotoFromAlbum(r.Context(), int32(albumPhotoID), user.ID); err != nil {
		if errors.Is(err, domain.ErrPhotoNotFound) {
			a.flash(r.Context(), http.StatusText(http.StatusNotFound), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		a.Logger.Error("error removing photo from album: %v", err)
		a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.flash(r.Context(), "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func (a *application) aboutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	td := a.generateTemplateData(r)
	a.renderTemplate(w, td, "about.html")
}
