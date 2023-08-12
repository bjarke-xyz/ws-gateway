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
	"github.com/google/uuid"
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
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			token, _, _ := TokenFromContext(r.Context())
			apps, err := s.appRepository.GetByUserID(r.Context(), token.Subject)
			errMsgs := make([]string, 0)
			if err != nil {
				s.logger.Error("error getting apps by user id", "error", err, "userId", token.Subject)
				errMsgs = append(errMsgs, "Error getting apps")
			}
			keys, err := s.keyRepository.GetByUserID(r.Context(), token.Subject)
			if err != nil {
				s.logger.Error("error getting keys by user id", "error", err, "userId", token.Subject)
				errMsgs = append(errMsgs, "Error getting keys")
			}
			appsByID := make(map[string]domain.Application)
			for _, v := range apps {
				appsByID[v.ID] = v
			}
			params := html.AdminParams{
				Title:    "Admin",
				Errors:   errMsgs,
				Apps:     apps,
				AppsByID: appsByID,
				Keys:     keys,
			}
			html.AdminPage(w, params)
		})
		r.Get("/app/{app-id}", func(w http.ResponseWriter, r *http.Request) {
			token, _, _ := TokenFromContext(r.Context())
			appId := chi.URLParam(r, "app-id")
			errMsg := r.URL.Query().Get("error")
			var app domain.Application
			var err error
			if appId != "null" {
				app, err = s.appRepository.GetByID(r.Context(), appId)
				if err != nil {
					s.logger.Error("error getting apps by user id", "error", err)
					errMsg = errMsg + " error getting apps"
				}
				if app.OwnerUserID != token.Subject {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			params := html.AppParams{
				Title: "App",
				Error: errMsg,
				App:   app,
			}
			html.AppPage(w, params)
		})
		r.Post("/app/{app-id}", func(w http.ResponseWriter, r *http.Request) {
			token, _, _ := TokenFromContext(r.Context())
			errMsg := ""
			appId := chi.URLParam(r, "app-id")
			name := r.FormValue("name")
			delete := r.FormValue("delete") == "true"
			if appId == "null" {
				appId = uuid.NewString()
				app := domain.Application{
					ID:          appId,
					OwnerUserID: token.Subject,
					Name:        name,
				}
				err := s.appRepository.Create(r.Context(), &app)
				if err != nil {
					s.logger.Error("error getting apps by user id", "error", err)
					errMsg = "failed to create"
				}
			} else {
				app, err := s.appRepository.GetByID(r.Context(), appId)
				if err != nil {
					s.logger.Error("error getting apps by user id", "error", err)
				}
				if app.OwnerUserID != token.Subject {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if delete {
					err = s.appRepository.Delete(r.Context(), app.ID)
					if err != nil {
						s.logger.Error("failed to delete app", "error", err)
						errMsg = "Failed to delete"
					}
				} else {
					app.Name = name
					err = s.appRepository.Update(r.Context(), &app)
					if err != nil {
						s.logger.Error("failed to update app", "error", err)
						errMsg = "Failed to update"
					}
				}
			}
			if delete {
				http.Redirect(w, r, fmt.Sprintf("/admin/?%v", errorQuery(errMsg)), http.StatusSeeOther)
			} else {
				http.Redirect(w, r, fmt.Sprintf("/admin/app/%v?%v", appId, errorQuery(errMsg)), http.StatusSeeOther)
			}
		})

	})
	return r
}
