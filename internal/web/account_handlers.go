package web

import (
	"errors"
	"net/http"

	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/forms"
)

func (a *application) signupHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			a.flash(r.Context(), "Error processing form. Please try again.", flashErr)
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
			a.renderTemplate(w, td, "signup.html")
			return
		}

		if _, err := a.userService.CreateUser(r.Context(), sf.FirstName, sf.LastName, sf.Email, sf.Password); err != nil {
			a.ErrorLog.Println(err)
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		a.flash(r.Context(), "Account successfully created.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case http.MethodGet:
		td := a.generateTemplateData(r)
		td.Form = &forms.SignupForm{}

		a.renderTemplate(w, td, "signup.html")
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.flash(r.Context(), http.StatusText(http.StatusBadRequest), flashErr)
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
			a.renderTemplate(w, td, "login.html")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		user, err := a.userService.GetUserByEmail(r.Context(), lf.Email)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				a.flash(r.Context(), "No account found with that email address.", flashErr)
				td := a.generateTemplateData(r)
				td.Form = lf

				w.WriteHeader(http.StatusForbidden)
				a.renderTemplate(w, td, "login.html")
			} else {
				a.ErrorLog.Printf("error getting user by email: %s", err)
				a.flash(r.Context(), "Internal server error.", flashErr)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}

		if !a.userService.Authenticate(user.PasswordHash, lf.Password) {
			a.flash(r.Context(), "Incorrect password.", flashErr)

			td := a.generateTemplateData(r)
			td.Form = lf

			a.renderTemplate(w, td, "login.html")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := a.sessionManager.RenewToken(r.Context()); err != nil {
			a.ErrorLog.Printf("error renewing token: %s", err)
			a.flash(r.Context(), "Internal server error.", flashErr)
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
		a.renderTemplate(w, td, "login.html")
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
			a.flash(r.Context(), "Error logging out. Please try again.", flashErr)
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
		a.renderTemplate(w, td, "profile.html")
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (a *application) editProfileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		td := a.generateTemplateData(r)
		td.AddPhotoUploadAction = "/photos?type=user"
		td.Form = &forms.EditProfileForm{
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		}

		a.renderTemplate(w, td, "edit-profile.html")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			a.ErrorLog.Println("error parsing form:", err)
			a.flash(r.Context(), "Error processing form. Please try again.", flashErr)
			http.Redirect(w, r, "/profile/edit", http.StatusSeeOther)
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
			a.renderTemplate(w, td, "edit-profile.html")
			return
		}

		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		_, err := a.userService.UpdateUser(r.Context(), user.ID, epf.FirstName, epf.LastName, epf.Email, epf.Password)
		if err != nil {
			a.ErrorLog.Printf("error updating user %d: %s", user.ID, err.Error())
			a.flash(r.Context(), "Error updating profile. Please try again.", flashErr)
			http.Redirect(w, r, "/profile/edit", http.StatusSeeOther)
			return
		}

		a.flash(r.Context(), "Successfully updated profile.", flashInfo)
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}
}

func (a *application) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		user := a.extractUserFromRequest(r)
		if user == nil {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Delete user account. This cascades to delete all albums and album_photos entries. Photos are not immediately deleted,
		// but will be cleaned up by the storage cleaner worker.
		if err := a.userService.DeleteUser(r.Context(), user.ID); err != nil {
			a.ErrorLog.Println("error deleting user:", err)
			a.flash(r.Context(), "Error deleting account. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.ErrorLog.Println("error deleting session:", err)
		}

		a.flash(r.Context(), "Your account has been deleted.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
