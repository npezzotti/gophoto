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
	FormFileName = "file"
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
				a.ErrorLog.Println("error converting string to int", err)
				a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			paginator := pagination.NewPaginationFromRequest(r)

			albumPageView, err := a.albumService.GetAlbumPageView(r.Context(), user.ID, int32(id), int32(paginator.Limit), int32(paginator.Offset()))
			if err != nil {
				var errMsg string
				if errors.Is(err, domain.ErrAlbumNotFound) {
					errMsg = http.StatusText(http.StatusNotFound)
				} else {
					errMsg = http.StatusText(http.StatusInternalServerError)
				}
				a.ErrorLog.Println("error getting album page view:", err)
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
			a.ErrorLog.Printf("error listing albums: %s", err.Error())
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
			if err := r.ParseForm(); err != nil {
				a.ErrorLog.Println("error parsing form:", err)
				a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		album, err := a.albumService.CreateAlbum(r.Context(), user.ID, r.Form.Get("title"))
		if err != nil {
			a.ErrorLog.Println("error creating album:", err)
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
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
			if err := r.ParseForm(); err != nil {
				a.ErrorLog.Println("error parsing form:", err)
				a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		albumIDStr := r.URL.Query().Get("id")
		albumID, err := strconv.Atoi(albumIDStr)
		if err != nil {
			a.ErrorLog.Println("error converting string to int:", err)
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

		if _, err := a.albumService.UpdateAlbum(r.Context(), int32(albumID), user.ID, r.Form.Get("title"), nil); err != nil {
			if errors.Is(err, domain.ErrAlbumNotFound) {
				a.flash(r.Context(), http.StatusText(http.StatusNotFound), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
			a.ErrorLog.Println("error updating album:", err)
			a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.flash(r.Context(), "User not found.", flashErr)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	albumIDStr := r.URL.Query().Get("id")
	if albumIDStr == "" {
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}
	albumID, err := strconv.Atoi(albumIDStr)
	if err != nil {
		a.ErrorLog.Println("error converting string to int", err)
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	if err := a.albumService.DeleteAlbum(r.Context(), int32(albumID), user.ID); err != nil {
		if errors.Is(err, domain.ErrAlbumNotFound) {
			a.flash(r.Context(), http.StatusText(http.StatusNotFound), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		a.ErrorLog.Printf("error deleting album: %v", err)
		a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
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
		a.writeJsonErrorResp(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	photoType := PhotoType(strings.ToLower(r.URL.Query().Get("type")))
	if photoType == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "missing \"type\" query parameter")
		return
	}

	if err := r.ParseForm(); err != nil {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "error parsing form")
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
			a.ErrorLog.Printf("error creating photo: %s", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	case PhotoTypeUserPhoto:
		photo, err = a.photoService.CreateUserPhotoWithOriginalMetadata(r.Context(), f, fh, user.ID)
		if err != nil {
			a.ErrorLog.Printf("error creating photo: %s", err)
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
		a.writeJsonErrorResp(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	user, ok := extractUserFromContext(r.Context())
	if !ok {
		a.writeJsonErrorResp(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error converting string to int", err)
		a.writeJsonErrorResp(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	photo, err := a.photoService.GetPhoto(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, domain.ErrPhotoNotFound) {
			a.writeJsonErrorResp(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		a.ErrorLog.Println("error getting photo:", err)
		a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	if photo.UserID == nil || *photo.UserID != user.ID {
		a.writeJsonErrorResp(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}

	a.writeJsonResp(w, http.StatusOK, map[string]string{"status": string(photo.Status)})
}

func (a *application) deleteAlbumPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	photoIDStr := r.URL.Query().Get("id")
	if photoIDStr == "" {
		a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	albumPhotoID, err := strconv.Atoi(photoIDStr)
	if err != nil {
		a.ErrorLog.Println("error parsing id:", err)
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

	// Set album_id to null to mark photo for deletion by storage cleanup worker
	albumPhoto, err := a.photoService.RemovePhotoFromAlbum(r.Context(), int32(albumPhotoID), user.ID)
	if err != nil {
		a.ErrorLog.Println("error removing photo from album:", err)
		a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.flash(r.Context(), "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", albumPhoto.AlbumID), http.StatusSeeOther)
}

func (a *application) aboutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	td := a.generateTemplateData(r)
	a.renderTemplate(w, td, "about.html")
}
