package server

import (
	"fmt"
	"net/http"

	"github.com/bjarke-xyz/ws-gateway/internal/domain"
	"github.com/bjarke-xyz/ws-gateway/internal/server/html"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *server) handleGetAdmin(w http.ResponseWriter, r *http.Request) {
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
}

func (s *server) handleGetApp(w http.ResponseWriter, r *http.Request) {
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
}

func (s *server) handlePostApp(w http.ResponseWriter, r *http.Request) {
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
}
