package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/utils"
	"github.com/npezzotti/gophoto/internal/workers"
	"github.com/npezzotti/gophoto/pkg/forms"
)

type UserResponse struct {
	FirstName               string
	LastName                string
	Email                   string
	ProfilePicture          ImageResponse
	ProfilePictureThumbURL  string
	ProfilePictureAvatarURL string
}

// getUserFromRequest retrieves the authenticated user's details from the request context.
func (a *application) getUserFromRequest(r *http.Request) *db.GetUserByIdRow {
	if userId, ok := r.Context().Value(AuthenticatedUserId).(int32); ok {
		userRow, err := a.database.GetUserById(r.Context(), userId)
		if err != nil {
			a.ErrorLog.Printf("error getting user by id from request: %s", err.Error())
			if err != sql.ErrNoRows {
				a.ErrorLog.Printf("error querying user: %s", err.Error())
			}
			return nil
		}
		return &userRow
	}

	return nil
}

func (a *application) newUserResponse(ctx context.Context, user *db.GetUserByIdRow) *UserResponse {
	var sources []Image
	var defaultSrc string
	if user.ProfilePictureKey.Valid {
		meta, err := a.database.GetPhotoMetadataByPhotoID(ctx, user.ProfilePictureID.Int32)
		if err != nil {
			a.ErrorLog.Printf("error getting photo metadata for user profile picture: %s", err)
		}

		for _, m := range meta {
			if m.Variant == db.PhotoVariantOriginal {
				continue
			}

			path, err := utils.BuildPhotoPath(user.ProfilePictureKey.String, m.Variant, utils.MimeType(m.MimeType))
			if err != nil {
				a.ErrorLog.Printf("error building photo path for user profile picture: %s", err)
				continue
			}

			url, err := a.store.GenerateURL(ctx, path)
			if err != nil {
				a.ErrorLog.Printf("error generating url for user profile picture: %s", err)
				continue
			}

			sources = append(sources, Image{
				Width:  m.Width,
				Height: m.Height,
				URL:    url,
			})

			if m.Variant == db.PhotoVariantSmall {
				defaultSrc = url
			}
		}
	}

	if len(sources) == 0 {
		thumbnailPath := filepath.Join(a.config.StaticDir, DefaultProfileThumbnailPath)
		sources = append(sources,
			Image{
				Width:  300,
				Height: 300,
				URL:    thumbnailPath,
			},
			Image{
				Width:  100,
				Height: 100,
				URL:    filepath.Join(a.config.StaticDir, DefaultProfileAvatarPath),
			},
		)

		defaultSrc = thumbnailPath
	}

	return &UserResponse{
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		ProfilePicture: ImageResponse{
			Alt:        fmt.Sprintf("%s %s's profile picture", user.FirstName, user.LastName),
			Sources:    sources,
			DefaultSrc: defaultSrc,
		},
	}
}

func (a *application) signupHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			a.flash(r, "Error processing form. Please try again.", flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		sf := &forms.SignupForm{
			FirstName:       r.Form.Get("first_name"),
			LastName:        r.Form.Get("last_name"),
			Email:           r.Form.Get("email"),
			Password:        r.Form.Get("password"),
			ConfirmPassword: r.Form.Get("confirm_password"),
		}

		if !sf.Validate() {
			td := a.generateTemplateData(r)
			td.Form = sf

			w.WriteHeader(http.StatusForbidden)
			if err := a.renderTemplate(w, td, "signup.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}

		passwdHash, err := hashPassword(sf.Password)
		if err != nil {
			a.ErrorLog.Println("error hashing password:", err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		user_params := db.CreateUserParams{
			FirstName:    sf.FirstName,
			LastName:     sf.LastName,
			Email:        sf.Email,
			PasswordHash: passwdHash,
		}
		_, err = a.database.CreateUser(r.Context(), user_params)
		if err != nil {
			a.ErrorLog.Println(err)
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		a.flash(r, "Account successfully created.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case http.MethodGet:
		td := a.generateTemplateData(r)
		td.Form = &forms.SignupForm{}

		if err := a.renderTemplate(w, td, "signup.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		lf := &forms.LoginForm{
			Email:    r.Form.Get("email"),
			Password: r.Form.Get("password"),
		}

		if !lf.Validate() {
			td := a.generateTemplateData(r)
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
				a.flash(r, "No account found with that email address.", flashErr)
				td := a.generateTemplateData(r)
				td.Form = lf

				w.WriteHeader(http.StatusForbidden)
				if err := a.renderTemplate(w, td, "login.html"); err != nil {
					a.ErrorLog.Printf("error rendering template: %s", err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			} else {
				a.ErrorLog.Printf("error getting user by email: %s", err)
				a.flash(r, "Internal server error.", flashErr)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}

		if !passwordsMatch(user.PasswordHash, lf.Password) {
			a.flash(r, "Incorrect password.", flashErr)

			td := a.generateTemplateData(r)
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
			a.flash(r, "Internal server error.", flashErr)
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
		td := a.generateTemplateData(r)
		td.Form = &forms.LoginForm{}
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

func (a *application) logoutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.ErrorLog.Println("error deleting session:", err)
			a.flash(r, "Error logging out. Please try again.", flashErr)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) profileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		td := a.generateTemplateData(r)
		if err := a.renderTemplate(w, td, "profile.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) editProfileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.getUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		td := a.generateTemplateData(r)
		td.AddPhotoUploadAction = "/profile/photo/edit"
		td.Form = &forms.EditProfileForm{
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		}

		if err := a.renderTemplate(w, td, "edit-profile.html"); err != nil {
			a.ErrorLog.Printf("error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			a.flash(r, "Error processing form. Please try again.", flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		epf := &forms.EditProfileForm{
			FirstName:       r.PostFormValue("first_name"),
			LastName:        r.PostFormValue("last_name"),
			Email:           r.PostFormValue("email"),
			Password:        r.PostFormValue("password"),
			ConfirmPassword: r.PostFormValue("confirm_password"),
		}

		if !epf.Validate() {
			td := a.generateTemplateData(r)
			td.Form = epf
			if err := a.renderTemplate(w, td, "edit-profile.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}

		user := a.getUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		pwdHash := user.PasswordHash
		if epf.Password != "" {
			// User wants to change their password
			hash, err := hashPassword(epf.Password)
			if err != nil {
				a.ErrorLog.Println("error hashing password:", err)
				a.flash(r, "Error updating profile. Please try again.", flashErr)
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
				return
			}
			pwdHash = hash
		}

		_, err := a.database.UpdateUser(r.Context(), db.UpdateUserParams{
			ID:               user.ID,
			FirstName:        epf.FirstName,
			LastName:         epf.LastName,
			Email:            epf.Email,
			PasswordHash:     pwdHash,
			ProfilePictureID: sql.NullInt32{Int32: user.ProfilePictureID.Int32, Valid: user.ProfilePictureID.Valid},
			UpdatedAt:        time.Now(),
		})
		if err != nil {
			a.ErrorLog.Printf("error updating user %d: %s", user.ID, err.Error())
			a.flash(r, "Error updating profile. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		a.flash(r, "Successfully updated profile.", flashInfo)
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}
}

func (a *application) editProfilePictureHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		f, fh, err := r.FormFile(FormFileName)
		if err != nil {
			if err := a.writeJsonResp(w, http.StatusBadRequest, map[string]string{"error": strings.ToLower(http.StatusText(http.StatusBadRequest))}); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}
		defer f.Close()

		if fh.Size > MaxUploadSize {
			if err := a.writeJsonResp(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Profile picture exceeds maximum upload size of %dMB.", MaxUploadSize/1024/1024)}); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		filetype, err := detectContentType(f)
		if err != nil {
			a.ErrorLog.Println("error detecting content type:", err)
			resp := map[string]string{"error": strings.ToLower(http.StatusText(http.StatusInternalServerError))}
			if err := a.writeJsonResp(w, http.StatusInternalServerError, resp); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}

		if !strings.HasPrefix(filetype, "image/") || !utils.ValidateMimeType(filetype) {
			resp := map[string]string{"error": "file type not allowed"}
			if err := a.writeJsonResp(w, http.StatusBadRequest, resp); err != nil {
				a.ErrorLog.Println("error writing json response:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}

		key := uuid.New().String()

		user := a.getUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		createPhotoParams := db.CreatePhotoParams{
			UserID: sql.NullInt32{Int32: user.ID, Valid: true},
			Key:    key,
			Status: db.PhotoStatusProcessing,
		}
		photo, err := a.database.CreatePhoto(r.Context(), createPhotoParams)
		if err != nil {
			a.ErrorLog.Printf("error creating photo record: %s", err)
			a.flash(r, "Error uploading profile picture. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		if _, err = a.database.CreatePhotoMetadata(r.Context(), db.CreatePhotoMetadataParams{
			PhotoID:  photo.ID,
			Variant:  db.PhotoVariantOriginal,
			FileSize: sql.NullInt64{Int64: fh.Size, Valid: true},
			MimeType: filetype,
		}); err != nil {
			a.ErrorLog.Printf("error creating photo metadata record: %s", err)
			a.flash(r, "Error uploading profile picture. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		path, err := utils.BuildPhotoPath(key, db.PhotoVariantOriginal, utils.MimeType(filetype))
		if err != nil {
			a.ErrorLog.Printf("error building photo path: %s", err)
			a.flash(r, "Error uploading profile picture. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}
		if err := a.store.Write(r.Context(), path, f); err != nil {
			a.ErrorLog.Printf("error writing profile picture to storage: %s", err)
			a.flash(r, "Error uploading profile picture. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		// Process photo in background
		processingJob, err := json.Marshal(workers.PhotoProcessingJob{Type: workers.JobTypeUserPhoto, PhotoID: photo.ID})
		if err != nil {
			a.ErrorLog.Printf("error marshalling photo processing job: %s", err.Error())
			return
		}
		err = a.redisClient.Publish(context.Background(), workers.PhotoProcessingQueue, processingJob).Err()
		if err != nil {
			a.ErrorLog.Printf("error publishing photo processing job: %s", err.Error())
			return
		}

		_, err = a.database.UpdateUser(r.Context(), db.UpdateUserParams{
			ID:               user.ID,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			Email:            user.Email,
			PasswordHash:     user.PasswordHash,
			ProfilePictureID: sql.NullInt32{Int32: photo.ID, Valid: true},
			UpdatedAt:        time.Now(),
		})
		if err != nil {
			a.ErrorLog.Println("error updating user:", err)
			a.flash(r, "Error updating profile picture, Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]int32{"id": photo.ID}); err != nil {
			a.ErrorLog.Println("error encoding json:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.getUserFromRequest(r)
		if user == nil {
			a.flash(r, "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Delete user account. This cascades to delete all albums and album_photos entries. Photos are not immediately deleted,
		// but will be cleaned up by the storage cleaner worker.
		if err := a.database.DeleteUser(r.Context(), user.ID); err != nil {
			a.ErrorLog.Println("error deleting user:", err)
			a.flash(r, "Error deleting account. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.ErrorLog.Println("error deleting session:", err)
		}

		a.flash(r, "Your account has been deleted.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
