package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bjarke-xyz/ws-gateway/internal/domain"
	"github.com/bjarke-xyz/ws-gateway/internal/server/html"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *server) truncateKeyPreviews(ctx context.Context, keys []domain.ApiKey) error {
	for _, key := range keys {
		if len(key.KeyPreview) > 4 {
			keyPreview := key.KeyPreview[0:4]
			err := s.keyRepository.UpdateKeyPreview(ctx, key.ID, keyPreview)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

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
	err = s.truncateKeyPreviews(r.Context(), keys)
	if err != nil {
		s.logger.Error("error truncating key preview", "error", err)
		errMsgs = append(errMsgs, "Error truncating key preview")
	}
	appsByID := make(map[string]domain.Application)
	for _, v := range apps {
		appsByID[v.ID] = v
	}
	params := html.AdminParams{
		Title:    "ws-gateway",
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
			s.logger.Error("error creating app", "error", err, "app", app)
			errMsg = "failed to create"
		}
	} else {
		app, err := s.appRepository.GetByID(r.Context(), appId)
		if err != nil {
			s.logger.Error("error getting app by app id", "error", err)
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
	redirectToAdmin(w, r, errMsg)
}

func (s *server) handleGetKey(w http.ResponseWriter, r *http.Request) {
	token, _, _ := TokenFromContext(r.Context())
	keyId := chi.URLParam(r, "key-id")
	errMsgs := make([]string, 0)
	errMsg := r.URL.Query().Get("error")
	if errMsg != "" {
		errMsgs = append(errMsgs, errMsg)
	}
	var key domain.ApiKey
	var err error
	if keyId != "null" {
		key, err = s.keyRepository.GetByID(r.Context(), keyId)
		if err != nil {
			s.logger.Error("error getting apps by user id", "error", err)
			errMsgs = append(errMsgs, "error getting apps")
		}
		if key.OwnerUserID != token.Subject {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	apps, err := s.appRepository.GetByUserID(r.Context(), token.Subject)
	if err != nil {
		s.logger.Error("error getting apps by user id", "error", err, "userId", token.Subject)
		errMsgs = append(errMsgs, "Error getting apps")
	}
	keyAccessByAppID := make(map[string]domain.ApiKeyAccess)
	for _, access := range key.Access {
		keyAccessByAppID[access.AppID] = access
	}
	params := html.KeyParams{
		Title:            "Key",
		Errors:           errMsgs,
		Key:              key,
		KeyAccessByAppID: keyAccessByAppID,
		Apps:             apps,
	}
	html.KeyPage(w, params)
}

func redirectToAdmin(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, fmt.Sprintf("/admin/?%v", errorQuery(errMsg)), http.StatusSeeOther)
}

func (s *server) handlePostKey(w http.ResponseWriter, r *http.Request) {
	token, _, _ := TokenFromContext(r.Context())
	errMsg := ""
	keyId := chi.URLParam(r, "key-id")
	delete := r.FormValue("delete") == "true"
	apps, err := s.appRepository.GetByUserID(r.Context(), token.Subject)
	if err != nil {
		s.logger.Error("error getting apps by user id", "error", err, "userId", token.Subject)
		redirectToAdmin(w, r, "error getting apps")
		return
	}
	apiKeyAccess := make([]domain.ApiKeyAccess, 0)
	r.ParseForm()
	formApps := r.Form["apps"]
	formAppsMap := make(map[string]bool)
	for _, v := range formApps {
		formAppsMap[v] = true
	}
	for _, app := range apps {
		_, ok := formAppsMap[app.ID]
		if ok {
			access := domain.ApiKeyAccess{
				AppID: app.ID,
			}
			apiKeyAccess = append(apiKeyAccess, access)
		}
	}
	if keyId == "null" {
		keyId = uuid.NewString()
		apiKey := uuid.NewString()
		apiKeyHashBytes, err := bcrypt.GenerateFromPassword([]byte(apiKey), 14)
		if err != nil {
			s.logger.Error("error hashing apiKey", "error", err)
			redirectToAdmin(w, r, "failed to hash api key")
			return
		}
		apiKeyHash := string(apiKeyHashBytes)
		key := domain.ApiKey{
			ID:          keyId,
			OwnerUserID: token.Subject,
			KeyHash:     apiKeyHash,
			// KeyPreview is truncated next time the api key is fetched
			KeyPreview: apiKey,
			Access:     apiKeyAccess,
		}
		err = s.keyRepository.Create(r.Context(), &key)
		if err != nil {
			s.logger.Error("error creating key", "error", err, "key", key)
			errMsg = "failed to create"
		}
	} else {
		key, err := s.keyRepository.GetByID(r.Context(), keyId)
		if err != nil {
			s.logger.Error("error getting key by id", "error", err, "keyId", keyId)
		}
		if key.OwnerUserID != token.Subject {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if delete {
			err = s.keyRepository.Delete(r.Context(), key.ID)
			if err != nil {
				s.logger.Error("failed to delete key", "error", err, "keyId", key.ID)
				errMsg = "Failed to delete"
			}
		} else {
			err = s.keyRepository.Update(r.Context(), key.ID, apiKeyAccess)
			if err != nil {
				s.logger.Error("failed to update key", "error", err)
				errMsg = "Failed to update"
			}
		}
	}
	redirectToAdmin(w, r, errMsg)
}
