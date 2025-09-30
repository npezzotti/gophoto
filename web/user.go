package web

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/npezzotti/gophoto/db"
)

type UserResponse struct {
	FirstName         string
	LastName          string
	Email             string
	ProfilePictureURL string
}

func (a *application) newUserResponse(ctx context.Context, user *db.User) *UserResponse {
	var url = filepath.Join("/assets", DefaultProfilePic)

	if user.ProfilePictureKey.Valid {
		existingUrl, err := a.store.Read(ctx, user.ProfilePictureKey.String)
		if err == nil {
			url = existingUrl
		}
	}

	return &UserResponse{
		FirstName:         user.FirstName,
		LastName:          user.LastName,
		Email:             user.Email,
		ProfilePictureURL: url,
	}
}

func (a *application) signupHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			a.Flash(r, "Error processing form. Please try again.", flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		sf := &SignupForm{
			FirstName:       r.Form.Get("first_name"),
			LastName:        r.Form.Get("last_name"),
			Email:           r.Form.Get("email"),
			Password:        r.Form.Get("password"),
			ConfirmPassword: r.Form.Get("confirm_password"),
		}

		if !sf.Validate() {
			td := a.newTemplateData(r)
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
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusInternalServerError)), flashErr)
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
			a.Flash(r, strings.ToLower(http.StatusText(http.StatusBadRequest)), flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		a.Flash(r, "Account successfully created.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case http.MethodGet:
		td := a.newTemplateData(r)
		td.Form = &SignupForm{}

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

func (a *application) logoutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.ErrorLog.Println("error deleting session:", err)
			a.Flash(r, "Error logging out. Please try again.", flashErr)
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
		td := a.newTemplateData(r)
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
		td := a.newTemplateData(r)
		td.Form = &EditProfileForm{
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
			a.Flash(r, "Error processing form. Please try again.", flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		epf := &EditProfileForm{
			FirstName:       r.PostFormValue("first_name"),
			LastName:        r.PostFormValue("last_name"),
			Email:           r.PostFormValue("email"),
			Password:        r.PostFormValue("password"),
			ConfirmPassword: r.PostFormValue("confirm_password"),
		}

		if !epf.Validate() {
			td := a.newTemplateData(r)
			td.Form = epf
			if err := a.renderTemplate(w, td, "edit-profile.html"); err != nil {
				a.ErrorLog.Printf("error rendering template: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			return
		}

		user := a.getUserFromRequest(r)
		pwdHash := user.PasswordHash
		if epf.Password != "" {
			// User wants to change their password
			hash, err := hashPassword(epf.Password)
			if err != nil {
				a.ErrorLog.Println("error hashing password:", err)
				a.Flash(r, "Error updating profile. Please try again.", flashErr)
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
				return
			}
			pwdHash = hash
		}

		_, err := a.database.UpdateUser(r.Context(), db.UpdateUserParams{
			ID:           user.ID,
			FirstName:    epf.FirstName,
			LastName:     epf.LastName,
			Email:        epf.Email,
			PasswordHash: pwdHash,
			UpdatedAt:    time.Now(),
		})
		if err != nil {
			a.ErrorLog.Printf("error updating user %d: %s\n", user.ID, err.Error())
			a.Flash(r, "Error updating profile. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		a.Flash(r, "Successfully updated profile.", flashInfo)
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
			if err := r.ParseForm(); err != nil {
				a.ErrorLog.Println("error parsing form:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		file, fileHeader, err := r.FormFile("profile_picture")
		if err != nil {
			a.Flash(r, "Error uploading profile picture. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}
		defer file.Close()

		if fileHeader.Size > MaxUploadSize {
			a.Flash(r, "Profile picture exceeds maximum upload size of 5MB.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		user := a.getUserFromRequest(r)
		if user.ProfilePictureKey.Valid {
			if err := a.store.Delete(r.Context(), user.ProfilePictureKey.String); err != nil {
				a.ErrorLog.Printf("error deleting existing profile picture from storage: %s", err)
			}
		}

		key := uuid.New().String()
		if err := a.store.Write(r.Context(), key, file); err != nil {
			a.ErrorLog.Printf("error writing profile picture to storage: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		_, err = a.database.UpdateUser(r.Context(), db.UpdateUserParams{
			ID:                user.ID,
			FirstName:         user.FirstName,
			LastName:          user.LastName,
			Email:             user.Email,
			PasswordHash:      user.PasswordHash,
			ProfilePictureKey: sql.NullString{String: key, Valid: true},
			UpdatedAt:         time.Now(),
		})
		if err != nil {
			a.ErrorLog.Println("error updating user:", err)
			a.Flash(r, "Error updating profile picture, Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
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
		if err := a.database.DeleteUser(r.Context(), user.ID); err != nil {
			a.ErrorLog.Println("error deleting user:", err)
			a.Flash(r, "Error deleting account. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.ErrorLog.Println("error deleting session:", err)
		}

		a.Flash(r, "Your account has been deleted.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
