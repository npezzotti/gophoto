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
			a.Logger.Error("error parsing form: %v", err)
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
			a.renderTemplateWithStatus(w, td, http.StatusBadRequest, "signup.html")
			return
		}

		if _, err := a.userService.CreateUser(r.Context(), sf.FirstName, sf.LastName, sf.Email, sf.Password); err != nil {
			a.Logger.Error("error creating user: %v", err)
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
			a.renderTemplateWithStatus(w, td, http.StatusBadRequest, "login.html")
			return
		}

		user, err := a.userService.AuthenticateByEmail(r.Context(), lf.Email, lf.Password)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidCredentials) {
				a.flash(r.Context(), "Incorrect email or password.", flashErr)
				td := a.generateTemplateData(r)
				td.Form = lf
				a.renderTemplateWithStatus(w, td, http.StatusForbidden, "login.html")
				return
			} else {
				a.Logger.Error("error authenticating user: %v", err)
				a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}

		if err := a.sessionManager.RenewToken(r.Context()); err != nil {
			a.Logger.Error("error renewing token: %v", err)
			a.flash(r.Context(), http.StatusText(http.StatusInternalServerError), flashErr)
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
			a.Logger.Error("error deleting session: %v", err)
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
		user, ok := extractUserFromContext(r.Context())
		if !ok {
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
			a.Logger.Error("error parsing form: %v", err)
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
			a.renderTemplateWithStatus(w, td, http.StatusBadRequest, "edit-profile.html")
			return
		}

		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		_, err := a.userService.UpdateUser(r.Context(), user.ID, epf.FirstName, epf.LastName, epf.Email, epf.Password)
		if err != nil {
			a.Logger.Error("error updating user %d: %v", user.ID, err)
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
		user, ok := extractUserFromContext(r.Context())
		if !ok {
			a.flash(r.Context(), "User not found.", flashErr)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if err := a.userService.DeleteUser(r.Context(), user.ID); err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				a.flash(r.Context(), "User not found.", flashErr)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			a.Logger.Error("error deleting user with ID %d: %v", user.ID, err)
			a.flash(r.Context(), "Error deleting account. Please try again.", flashErr)
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		if err := a.sessionManager.Destroy(r.Context()); err != nil {
			a.Logger.Error("error deleting session: %v", err)
		}

		a.flash(r.Context(), "Your account has been deleted.", flashInfo)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
