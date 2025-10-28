package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/pkg/pagination"
)

const (
	DefaultProfileThumbnailPath = "images/profile_thumb.webp"
	DefaultProfileAvatarPath    = "images/profile_avatar.webp"
	DefaultAlbumCover           = "images/album_cover.webp"

	FormFileName = "file"
)

type Image struct {
	Width  int32
	Height int32
	URL    string
}

type Size struct {
	MediaQuery string
	Size       string
}

type ImageResponse struct {
	Image       db.Photo
	Alt         string
	OriginalSrc string
	DefaultSrc  string
	Sources     []Image
}

func (ar *ImageResponse) SrcSet() string {
	var srcset []string
	for _, source := range ar.Sources {
		srcset = append(srcset, fmt.Sprintf("%s %dw", source.URL, source.Width))
	}
	return strings.Join(srcset, ", ")
}

type AlbumResponse struct {
	Album           db.ListAlbumsByUserRow
	AlbumCoverImage ImageResponse
}

func (a *application) newAlbumResponse(ctx context.Context, album db.ListAlbumsByUserRow) *AlbumResponse {
	var imageResp ImageResponse
	if album.CoverPhotoID.Valid {
		meta, err := a.photoService.GetPhotoMetadataByPhotoID(ctx, album.CoverPhotoID.Int32)
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
	photoMeta, err := a.photoService.GetPhotoMetadataByPhotoID(ctx, photo.ID)
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
				a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			album, err := a.albumService.GetAlbumById(r.Context(), int32(id))
			if err != nil {
				a.ErrorLog.Println("error getting album:", err)
				a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			if user.ID != album.UserID {
				a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
				http.Redirect(w, r, "/albums", http.StatusSeeOther)
				return
			}

			pagination := pagination.NewPaginationFromRequest(r, int(album.NumPhotos))
			photos, err := a.photoService.ListPhotosByAlbum(r.Context(), album.ID, int32(pagination.Limit), int32(pagination.Offset()))
			if err != nil {
				a.flash(r, http.StatusText(http.StatusInternalServerError), flashErr)
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
		totalAlbums, err := a.albumService.CountAlbumsByUser(r.Context(), user.ID)
		if err != nil {
			a.ErrorLog.Println("error counting albums:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pagination := pagination.NewPaginationFromRequest(r, int(totalAlbums))

		albums, err := a.albumService.ListAlbumsByUser(r.Context(), user.ID, int32(pagination.Limit), int32(pagination.Offset()))
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
				a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
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

		album, err := a.albumService.CreateAlbum(r.Context(), user.ID, r.Form.Get("title"))
		if err != nil {
			a.ErrorLog.Println("error creating album:", err)
			a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
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
				a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
				http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
				return
			}
		}

		album_id_str := r.URL.Query().Get("id")
		album_id, err := strconv.Atoi(album_id_str)
		if err != nil {
			a.ErrorLog.Println("error converting string to int:", err)
			a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		album, err := a.albumService.GetAlbumById(r.Context(), int32(album_id))
		if err != nil {
			var flashMsg string
			if errors.Is(err, sql.ErrNoRows) {
				flashMsg = http.StatusText(http.StatusNotFound)
			} else {
				flashMsg = http.StatusText(http.StatusInternalServerError)
			}
			a.flash(r, flashMsg, flashErr)
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
			a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		if _, err := a.albumService.UpdateAlbum(r.Context(), album.ID, album.UserID, r.Form.Get("title"), album.CoverPhotoID); err != nil {
			a.ErrorLog.Println("error updating album:", err)
			a.flash(r, http.StatusText(http.StatusInternalServerError), flashErr)
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
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error converting string to int", err)
		a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	album, err := a.albumService.GetAlbumById(r.Context(), int32(id))
	if err != nil {
		var flashMsg string
		if errors.Is(err, sql.ErrNoRows) {
			flashMsg = http.StatusText(http.StatusNotFound)
		} else {
			flashMsg = http.StatusText(http.StatusInternalServerError)
		}
		a.flash(r, flashMsg, flashErr)
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
		a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	if err := a.albumService.DeleteAlbum(r.Context(), int32(album.ID)); err != nil {
		a.ErrorLog.Println("error deleting album:", err)
		a.flash(r, http.StatusText(http.StatusInternalServerError), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.flash(r, fmt.Sprintf("Successfully deleted album %q.", album.Title), flashInfo)
	http.Redirect(w, r, "/albums", http.StatusSeeOther)
}

func (a *application) uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user, err := a.authenticateRequest(r)
	if err != nil {
		a.ErrorLog.Printf("authentication error: %v", err)
		a.writeJsonErrorResp(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	photoType := PhotoType(r.URL.Query().Get("type"))
	if photoType == "" {
		a.writeJsonErrorResp(w, http.StatusBadRequest, "missing photo type")
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

	var photo db.Photo
	switch photoType {
	case PhotoTypeAlbumPhoto:
		album_id_str := r.URL.Query().Get("id")
		album_id, err := strconv.Atoi(album_id_str)
		if err != nil {
			a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid album id")
			return
		}

		album, err := a.albumService.GetAlbumById(r.Context(), int32(album_id))
		if err != nil {
			var respMsg string
			if errors.Is(err, sql.ErrNoRows) {
				respMsg = http.StatusText(http.StatusNotFound)
			} else {
				a.ErrorLog.Println("error retrieving album:", err)
				respMsg = http.StatusText(http.StatusInternalServerError)
			}
			a.writeJsonErrorResp(w, http.StatusBadRequest, respMsg)
			return
		}

		if user.ID != album.UserID {
			a.writeJsonErrorResp(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}

		photo, err = a.photoService.CreateAlbumPhotoWithOriginalMetadata(r.Context(), f, fh, user, album)
		if err != nil {
			a.ErrorLog.Printf("error creating photo: %s", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	case PhotoTypeUserPhoto:
		photo, err = a.photoService.CreateUserPhotoWithOriginalMetadata(r.Context(), f, fh, user)
		if err != nil {
			a.ErrorLog.Printf("error creating photo: %s", err)
			a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	default:
		a.writeJsonErrorResp(w, http.StatusBadRequest, "invalid photo type")
		return
	}

	a.writeJsonResp(w, http.StatusOK, map[string]int32{"id": photo.ID})
}

func (a *application) photoStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user, err := a.authenticateRequest(r)
	if err != nil {
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
		if errors.Is(err, sql.ErrNoRows) {
			a.writeJsonErrorResp(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		a.ErrorLog.Println("error getting photo:", err)
		a.writeJsonErrorResp(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	if !photo.UserID.Valid || photo.UserID.Int32 != user.ID {
		a.writeJsonErrorResp(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}

	a.writeJsonResp(w, http.StatusOK, map[string]string{"status": string(photo.Status)})
}

func (a *application) deletePhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	id_str := r.URL.Query().Get("id")
	if id_str == "" {
		a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		a.ErrorLog.Println("error parsing id:", err)
		a.flash(r, http.StatusText(http.StatusBadRequest), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	photo, err := a.photoService.GetAlbumPhoto(r.Context(), int32(id))
	if err != nil {
		a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
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
		a.flash(r, http.StatusText(http.StatusNotFound), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// Set album_id to null to mark photo for deletion by storage cleanup worker
	if err := a.photoService.RemovePhotoFromAlbum(r.Context(), photo.AlbumID, photo.ID); err != nil {
		a.ErrorLog.Println("error removing photo from album:", err)
		a.flash(r, http.StatusText(http.StatusInternalServerError), flashErr)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	a.flash(r, "Photo successfully deleted.", flashInfo)
	http.Redirect(w, r, fmt.Sprintf("/albums?id=%d", photo.AlbumID), http.StatusSeeOther)
}

func (a *application) aboutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	td := a.generateTemplateData(r)
	if err := a.renderTemplate(w, td, "about.html"); err != nil {
		a.ErrorLog.Printf("error rendering template: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
