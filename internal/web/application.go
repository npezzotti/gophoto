package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/internal/config"
	"github.com/npezzotti/gophoto/internal/db"
	"github.com/npezzotti/gophoto/internal/service"
	"github.com/npezzotti/gophoto/pkg/store"
	"github.com/npezzotti/gophoto/pkg/template"
	"github.com/redis/go-redis/v9"
)

type ContextKey string

const (
	AuthenticatedUserId       = ContextKey("authenticatedUserId")
	IsAuthenticatedContextKey = ContextKey("isAuthenticated")
)

const (
	SessionKeyRedirectPath = "redirectPath"
	SessionKeyUserID       = "userID"
	SessionKeyFlash        = "__flash"
)

type PhotoType string

const (
	PhotoTypeAlbumPhoto PhotoType = "album"
	PhotoTypeUserPhoto  PhotoType = "user"
)

type application struct {
	redisClient    *redis.Client
	config         *config.Config
	srv            *http.Server
	database       *db.Repository
	templateCache  template.TemplateCache
	sessionManager *scs.SessionManager
	InfoLog        *log.Logger
	ErrorLog       *log.Logger
	userService    *service.UserService
	albumService   *service.AlbumService
	photoService   *service.PhotoService
}

func NewApplication(redisClient *redis.Client, cfg *config.Config, sess *scs.SessionManager, db *db.Repository, s store.Store, tc template.TemplateCache) *application {
	app := &application{
		redisClient:    redisClient,
		config:         cfg,
		sessionManager: sess,
		database:       db,
		templateCache:  tc,
		userService:    service.NewUserService(db, s, cfg),
		albumService:   service.NewAlbumService(db, s, cfg),
		photoService:   service.NewPhotoService(db, s, redisClient),
	}

	app.InfoLog = log.New(os.Stdout, "[INFO] ", log.Default().Flags())
	app.ErrorLog = log.New(os.Stderr, "[ERROR] ", log.Default().Flags()|log.Lshortfile)

	mux := app.routes()

	app.srv = &http.Server{
		Addr:     cfg.HttpServerAddr,
		Handler:  setupMiddleware(mux, app.sessionManager.LoadAndSave, noSurf, app.authenticate),
		ErrorLog: app.ErrorLog,
	}

	return app
}

func (a *application) Start() error {
	return a.srv.ListenAndServe()
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
	mux.Handle("/photo/delete", a.protected(http.HandlerFunc(a.deletePhotoHandler)))
	mux.Handle("/api/photos", http.HandlerFunc(a.uploadPhotoHandler))
	mux.Handle("/api/photos/status", http.HandlerFunc(a.photoStatusHandler))
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
		mux.Handle("/uploads/", a.validatePresignedURL(a.protected(http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))))
	}

	return mux
}

// Authenticate and extract user
func (a *application) authenticateRequest(r *http.Request) (*service.User, error) {
	if !isAuthenticated(r) {
		return nil, fmt.Errorf("not authenticated")
	}

	user := a.extractUserFromRequest(r)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// extractUserFromRequest retrieves the authenticated user's details from the request context.
func (a *application) extractUserFromRequest(r *http.Request) *service.User {
	if userId, ok := r.Context().Value(AuthenticatedUserId).(int32); ok {
		userResp, err := a.userService.GetUserByID(r.Context(), userId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				a.ErrorLog.Printf("error querying user: %s", err.Error())
			}
			return nil
		}
		return userResp
	}

	return nil
}

// writeJsonResp writes the provided data as a JSON response with the specified HTTP status code.
func (a *application) writeJsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.ErrorLog.Println("error writing json response:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (a *application) writeJsonErrorResp(w http.ResponseWriter, status int, message string) {
	resp := map[string]string{"error": strings.ToLower(message)}
	a.writeJsonResp(w, status, resp)
}
