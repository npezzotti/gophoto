package domain

import "errors"

const (
	DefaultProfileThumbnailPath = "images/profile_thumb.webp"
	DefaultProfileAvatarPath    = "images/profile_avatar.webp"
)

var ErrUserNotFound = errors.New("user not found")

type UserPresentation struct {
	ID             int32
	FirstName      string
	LastName       string
	Email          string
	PasswordHash   string
	ProfilePicture ResponsiveImage
}

type User struct {
	ID                int32
	FirstName         string
	LastName          string
	Email             string
	PasswordHash      string
	ProfilePictureID  *int32
	ProfilePictureKey *string
}

type UserUpdateParams struct {
	ID               int32
	FirstName        string
	LastName         string
	Email            string
	PasswordHash     string
	ProfilePictureID *int32
}
