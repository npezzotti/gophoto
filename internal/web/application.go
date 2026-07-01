package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/domain"
	"github.com/npezzotti/gophoto/pkg/logging"
	"github.com/npezzotti/gophoto/pkg/template"
)

type ContextKey string

const (
	AuthenticatedUserContextKey = ContextKey("authenticatedUser")
	IsAuthenticatedContextKey   = ContextKey("isAuthenticated")
)

const (
	SessionKeyRedirectPath = "redirectPath"
	SessionKeyUserID       = "userID"
)

type PhotoType string

const (
	PhotoTypeAlbumPhoto PhotoType = "album"
	PhotoTypeUserPhoto  PhotoType = "user"
)

type albumService interface {
	GetAlbumPageView(ctx context.Context, userID, albumID, limit, offset int32) (domain.AlbumPageView, error)
	ListAlbumsByUser(ctx context.Context, userID int32, limit, offset int32) ([]*domain.AlbumListItem, error)
	CreateAlbum(context.Context, int32, string) (domain.Album, error)
	UpdateAlbum(ctx context.Context, userID, albumID int32, title string) (domain.Album, error)
	DeleteAlbum(ctx context.Context, userID, albumID int32) error
}

type photoService interface {
	CreateAlbumPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32, albumID int32) (domain.Photo, error)
	CreateUserPhotoWithOriginalMetadata(ctx context.Context, f multipart.File, fh *multipart.FileHeader, userID int32) (domain.Photo, error)
	GetPhoto(ctx context.Context, id int32) (domain.Photo, error)
	RemovePhotoFromAlbum(ctx context.Context, photoID, userID int32) error
}

type userService interface {
	GetUserByID(ctx context.Context, id int32) (*domain.UserPresentation, error)
	UpdateUser(ctx context.Context, userID int32, firstName, lastName, email, password string) (*domain.UserPresentation, error)
	DeleteUser(ctx context.Context, userID int32) error
	AuthenticateByEmail(ctx context.Context, email, password string) (domain.User, error)
	CreateUser(ctx context.Context, firstName, lastName, email, password string) (*domain.UserPresentation, error)
}

type application struct {
	config         *config.Config
	srv            *http.Server
	templateCache  template.TemplateCache
	sessionManager *scs.SessionManager
	Logger         *logging.Logger
	userService    userService
	albumService   albumService
	photoService   photoService
}

func NewApplication(userService userService, albumService albumService, photoService photoService, cfg *config.Config, sess *scs.SessionManager, tc template.TemplateCache, logger *logging.Logger) *application {
	app := &application{
		config:         cfg,
		sessionManager: sess,
		templateCache:  tc,
		Logger:         logger,
		userService:    userService,
		albumService:   albumService,
		photoService:   photoService,
	}

	mux := app.routes()

	app.srv = &http.Server{
		Addr:     cfg.HttpServerAddr,
		Handler:  setupMiddleware(mux, app.sessionManager.LoadAndSave, noSurf, app.authenticate),
		ErrorLog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
	}

	return app
}

func (a *application) Start() error {
	err := a.srv.ListenAndServe()
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *application) Shutdown(ctx context.Context) error {
	return a.srv.Shutdown(ctx)
}

func (a *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/albums", a.protected(http.HandlerFunc(a.getAlbumHandler)))
	mux.Handle("/albums/edit", a.protected(http.HandlerFunc(a.updateAlbumHandler)))
	mux.Handle("/albums/delete", a.protected(http.HandlerFunc(a.deleteAlbumHandler)))
	mux.Handle("/albums/new", a.protected(http.HandlerFunc(a.createAlbumHandler)))
	mux.Handle("/photo/delete", a.protected(http.HandlerFunc(a.deleteAlbumPhotoHandler)))
	mux.Handle("/photos", http.HandlerFunc(a.uploadPhotoHandler))
	mux.Handle("/photos/status", http.HandlerFunc(a.photoStatusHandler))
	mux.Handle("/login", http.HandlerFunc(a.loginHandler))
	mux.HandleFunc("/signup", a.signupHandler)
	mux.HandleFunc("/logout", a.logoutHandler)
	mux.HandleFunc("/about", a.aboutHandler)
	mux.Handle("/profile", a.protected(http.HandlerFunc(a.profileHandler)))
	mux.Handle("/profile/edit", a.protected(http.HandlerFunc(a.editProfileHandler)))
	mux.Handle("/profile/delete", a.protected(http.HandlerFunc(a.deleteAccountHandler)))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	if a.config.StorageType == config.StorageTypeDisk {
		// Only serve uploads directly if using local file storage
		mux.Handle(
			"/uploads/",
			a.validatePresignedURL(
				a.protected(
					http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.config.BaseDir))),
				),
			),
		)
	}

	return mux
}

// writeJsonResp writes the provided data as a JSON response with the specified HTTP status code.
func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.Logger.Error("error writing json response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (a *application) writeJsonErrorResp(w http.ResponseWriter, status int, message string) {
	resp := map[string]string{"error": strings.ToLower(message)}
	a.writeJsonResp(w, status, resp)
}
