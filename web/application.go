package web

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/alexedwards/scs/v2"
	"github.com/npezzotti/gophoto/config"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/store"
)

type application struct {
	config         *config.Config
	srv            *http.Server
	database       *db.Queries
	store          store.Store
	templateCache  map[string]*template.Template
	sessionManager *scs.SessionManager
	InfoLog        *log.Logger
	ErrorLog       *log.Logger
}

func NewApplication(cfg *config.Config, sess *scs.SessionManager, db *db.Queries, s store.Store, tc map[string]*template.Template) *application {
	app := &application{
		config:         cfg,
		sessionManager: sess,
		database:       db,
		templateCache:  tc,
		store:          s,
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
	mux.Handle("/photo/new", a.protected(http.HandlerFunc(a.createPhotoHandler)))
	mux.Handle("/login", http.HandlerFunc(a.loginHandler))
	mux.HandleFunc("/signup", a.signupHandler)
	mux.HandleFunc("/logout", a.logoutHandler)
	mux.HandleFunc("/about", a.aboutHandler)
	mux.Handle("/profile", a.protected(http.HandlerFunc(a.profileHandler)))
	mux.Handle("/profile/edit", a.protected(http.HandlerFunc(a.editProfileHandler)))
	mux.Handle("/profile/photo/edit", a.protected(http.HandlerFunc(a.editProfilePictureHandler)))
	mux.Handle("/profile/delete", a.protected(http.HandlerFunc(a.deleteAccountHandler)))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	return mux
}
