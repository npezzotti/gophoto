package forms_test

import (
	"testing"

	"github.com/npezzotti/gophoto/pkg/forms"
)

func TestLoginForm_Validate(t *testing.T) {
	tests := []struct {
		name     string
		lf       forms.LoginForm
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid form",
			lf: forms.LoginForm{
				Email:    "test@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "missing password",
			lf: forms.LoginForm{
				Email:    "test@example.com",
				Password: "",
			},
			wantErr:  true,
			errorMsg: "Password required",
		},
		{
			name: "missing email",
			lf: forms.LoginForm{
				Email:    "",
				Password: "password123",
			},
			wantErr:  true,
			errorMsg: "Email required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.lf.Validate()

			if tt.wantErr && got {
				t.Errorf("expected validation to fail, but it succeeded")
			}

			if tt.wantErr && tt.lf.Errors["Email"] != "" && tt.lf.Errors["Email"] != tt.errorMsg {
				t.Errorf("expected email error %q, got %q", tt.errorMsg, tt.lf.Errors["Email"])
			}

			if tt.wantErr && tt.lf.Errors["Password"] != "" && tt.lf.Errors["Password"] != tt.errorMsg {
				t.Errorf("expected password error %q, got %q", tt.errorMsg, tt.lf.Errors["Password"])
			}
		})
	}
}

func TestSignupForm_Validate(t *testing.T) {
	tests := []struct {
		name     string
		sf       forms.SignupForm
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid form",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr: false,
		},
		{
			name: "missing first name",
			sf: forms.SignupForm{
				FirstName:       "",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "First name required",
		},
		{
			name: "missing last name",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Last name required",
		},
		{
			name: "missing email",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Email required",
		},
		{
			name: "invalid email",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "invalid-email",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Please enter a valid email",
		},
		{
			name: "missing password",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Password required",
		},
		{
			name: "mismatched passwords",
			sf: forms.SignupForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password1321",
			},
			wantErr:  true,
			errorMsg: "Passwords do not match",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sf.Validate()

			if tt.wantErr && got {
				t.Errorf("expected validation to fail, but it succeeded")
			}

			if tt.wantErr && tt.sf.Errors["FirstName"] != "" && tt.sf.Errors["FirstName"] != tt.errorMsg {
				t.Log(tt.sf.Errors)
				t.Errorf("expected first name error %q, got %q", tt.errorMsg, tt.sf.Errors["FirstName"])
			}

			if tt.wantErr && tt.sf.Errors["LastName"] != "" && tt.sf.Errors["LastName"] != tt.errorMsg {
				t.Errorf("expected last name error %q, got %q", tt.errorMsg, tt.sf.Errors["LastName"])
			}

			if tt.wantErr && tt.sf.Errors["Email"] != "" && tt.sf.Errors["Email"] != tt.errorMsg {
				t.Errorf("expected email error %q, got %q", tt.errorMsg, tt.sf.Errors["Email"])
			}

			if tt.wantErr && tt.sf.Errors["Password"] != "" && tt.sf.Errors["Password"] != tt.errorMsg {
				t.Errorf("expected password error %q, got %q", tt.errorMsg, tt.sf.Errors["Password"])
			}

			if tt.wantErr && tt.sf.Errors["ConfirmPassword"] != "" && tt.sf.Errors["ConfirmPassword"] != tt.errorMsg {
				t.Errorf("expected mismatched passwords error %q, got %q", tt.errorMsg, tt.sf.Errors["ConfirmPassword"])
			}
		})
	}
}

func TestEditProfileForm_Validate(t *testing.T) {
	tests := []struct {
		name     string
		epf      forms.EditProfileForm
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid form",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr: false,
		},
		{
			name: "missing first name",
			epf: forms.EditProfileForm{
				FirstName:       "",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "First name required",
		},
		{
			name: "missing last name",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Last name required",
		},
		{
			name: "missing email",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Email required",
		},
		{
			name: "invalid email",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "invalid-email",
				Password:        "password123",
				ConfirmPassword: "password123",
			},
			wantErr:  true,
			errorMsg: "Please enter a valid email",
		},
		{
			name: "mismatched passwords",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "password123",
				ConfirmPassword: "password132",
			},
			wantErr:  true,
			errorMsg: "Passwords do not match",
		},
		{
			name: "missing password does not trigger error",
			epf: forms.EditProfileForm{
				FirstName:       "test",
				LastName:        "test",
				Email:           "test@example.com",
				Password:        "",
				ConfirmPassword: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.epf.Validate()

			if tt.wantErr && got {
				t.Errorf("expected validation to fail, but it succeeded")
			}

			if tt.wantErr && tt.epf.Errors["FirstName"] != "" && tt.epf.Errors["FirstName"] != tt.errorMsg {
				t.Errorf("expected first name error %q, got %q", tt.errorMsg, tt.epf.Errors["FirstName"])
			}

			if tt.wantErr && tt.epf.Errors["LastName"] != "" && tt.epf.Errors["LastName"] != tt.errorMsg {
				t.Errorf("expected last name error %q, got %q", tt.errorMsg, tt.epf.Errors["LastName"])
			}

			if tt.wantErr && tt.epf.Errors["Email"] != "" && tt.epf.Errors["Email"] != tt.errorMsg {
				t.Errorf("expected email error %q, got %q", tt.errorMsg, tt.epf.Errors["Email"])
			}

			if tt.wantErr && tt.epf.Errors["ConfirmPassword"] != "" && tt.epf.Errors["ConfirmPassword"] != tt.errorMsg {
				t.Errorf("expected mismatched passwords error %q, got %q", tt.errorMsg, tt.epf.Errors["ConfirmPassword"])
			}
		})
	}
}
