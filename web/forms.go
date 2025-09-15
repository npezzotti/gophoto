package web

import (
	"regexp"
)

const emailRegex = `^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$`

type Form interface {
	Validate() bool
}

type LoginForm struct {
	Email    string
	Password string
	Errors   map[string]string
}

func (lf *LoginForm) Validate() bool {
	lf.Errors = make(map[string]string)

	if lf.Email == "" {
		lf.Errors["Email"] = "Email required"
	}

	if lf.Password == "" {
		lf.Errors["Password"] = "Password required"
	}

	return len(lf.Errors) == 0
}

type SignupForm struct {
	FirstName       string
	LastName        string
	Email           string
	Password        string
	ConfirmPassword string
	Errors          map[string]string
}

func (sf *SignupForm) Validate() bool {
	sf.Errors = make(map[string]string)

	if sf.FirstName == "" {
		sf.Errors["FirstName"] = "First name required"
	}

	if sf.LastName == "" {
		sf.Errors["LastName"] = "Last name required"
	}

	if sf.Email == "" {
		sf.Errors["Email"] = "Email required"
	} else {
		matched, _ := regexp.Match(emailRegex, []byte(sf.Email))
		if !matched {
			sf.Errors["Email"] = "Please enter a valid email"
		}
	}

	if sf.Password == "" {
		sf.Errors["Password"] = "Password required"
	}

	if sf.Password != sf.ConfirmPassword {
		sf.Errors["ConfirmPassword"] = "Passwords do not match"
	}

	return len(sf.Errors) == 0
}

type EditProfileForm struct {
	FirstName       string
	LastName        string
	Email           string
	Password        string
	ConfirmPassword string
	Errors          map[string]string
}

func (epf *EditProfileForm) Validate() bool {
	epf.Errors = make(map[string]string)
	if epf.FirstName == "" {
		epf.Errors["FirstName"] = "First name required"
	}

	if epf.LastName == "" {
		epf.Errors["LastName"] = "Last name required"
	}

	if epf.Email == "" {
		epf.Errors["Email"] = "Email required"
	} else {
		matched, _ := regexp.Match(emailRegex, []byte(epf.Email))
		if !matched {
			epf.Errors["Email"] = "Please enter a valid email"
		}
	}

	if epf.Password != epf.ConfirmPassword {
		epf.Errors["ConfirmPassword"] = "Passwords do not match"
	}

	return len(epf.Errors) == 0
}
