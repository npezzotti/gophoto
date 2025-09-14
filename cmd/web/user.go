package main

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
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
	var url = filepath.Join("/assets", defaultProfilePic)

	if user.ProfilePictureKey.Valid {
		existingUrl, err := a.store.Read(user.ProfilePictureKey.String)
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
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		a.Flash(r, "Account created!", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case "GET":
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
	if err := a.sessionManager.Destroy(r.Context()); err != nil {
		a.ErrorLog.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
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
		user := a.getUserFromRequest(r)

		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		pwdHash := user.PasswordHash
		if epf.Password != "" {
			pwd, err := hashPassword(epf.Password)
			if err != nil {
				a.ErrorLog.Println("error hashing password:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			pwdHash = pwd
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		a.Flash(r, "Profile updated!", flashInfo)
		http.Redirect(w, r, "/profile", http.StatusSeeOther)

	default:
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}
}

func (a *application) editProfilePictureHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	file, _, err := r.FormFile("profile_picture")
	if err != nil {
		a.ErrorLog.Printf("error getting file from form: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	user := a.getUserFromRequest(r)

	if user.ProfilePictureKey.Valid {
		if err := a.store.Delete(user.ProfilePictureKey.String); err != nil {
			a.ErrorLog.Printf("error deleting photo from storage: %s", err)
		}
	}

	key := uuid.New().String()
	if err := a.store.Write(key, file); err != nil {
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (a *application) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	user := a.getUserFromRequest(r)

	if err := a.database.DeleteUser(r.Context(), user.ID); err != nil {
		a.ErrorLog.Println("error deleting user:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := a.sessionManager.Destroy(r.Context()); err != nil {
		a.ErrorLog.Println("error deleting session:", err)
	}

	a.Flash(r, "Your account has been deleted.", flashInfo)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
