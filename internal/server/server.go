package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	firebase "firebase.google.com/go/v4"
	"github.com/bjarke-xyz/ws-gateway/internal/domain"
	"github.com/bjarke-xyz/ws-gateway/internal/repository"
	"github.com/bjarke-xyz/ws-gateway/internal/server/html"
	"github.com/bjarke-xyz/ws-gateway/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed static
var staticFiles embed.FS

type server struct {
	logger *slog.Logger

	app        *firebase.App
	authClient *service.FirebaseAuthRestClient

	allowedUsers []string

	appRepository domain.ApplicationRepository
	keyRepository domain.ApiKeyRepository

	staticFilesFs fs.FS
}

func NewServer(ctx context.Context, logger *slog.Logger, app *firebase.App, authClient *service.FirebaseAuthRestClient, pool *pgxpool.Pool) (*server, error) {
	staticFilesFs, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	appRepo := repository.NewPostgresApp(pool)
	keyRepo := repository.NewPostgresKey(pool)
	return &server{
		logger:        logger,
		app:           app,
		authClient:    authClient,
		appRepository: appRepo,
		keyRepository: keyRepo,
		staticFilesFs: staticFilesFs,
	}, nil
}
func (s *server) Server(port int) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.routes(),
	}
}
func errorQuery(errMsg string) string {
	if errMsg == "" {
		return ""
	}
	return fmt.Sprintf("error=%v", errMsg)
}
func (s *server) routes() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFilesFs))))
	r.Handle("/favicon.ico", http.FileServer(http.FS(s.staticFilesFs)))
	r.Get("/up", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "up!")
	})

	r.Post("/login", s.handleLogin)
	r.Post("/logout", s.handleLogout)
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		err := r.URL.Query().Get("error")
		html.LoginPage(w, html.LoginParams{Title: "Login", Error: err})
	})
	r.Route("/admin", func(r chi.Router) {
		r.Use(s.firebaseJwtVerifier)
		r.Get("/", s.handleGetAdmin)

		r.Get("/app/{app-id}", s.handleGetApp)
		r.Post("/app/{app-id}", s.handlePostApp)

		r.Get("/key/{key-id}", s.handleGetKey)
		r.Post("/key/{key-id}", s.handlePostKey)

	})
	return r
}
