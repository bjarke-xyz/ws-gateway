package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"firebase.google.com/go/v4/errorutils"
	"github.com/go-chi/chi/v5"
)

const (
	wsTokenAppIdClaimKey = "app_id"
	wsTokenTopicClaimKey = "topic"
)

type createTicketInput struct {
	UserID string `json:"userId"`
	Topic  string `json:"topic"`
}
type createTicketResponse struct {
	Token string `json:"token"`
}

func (s *server) handleApiCreateTicket(w http.ResponseWriter, r *http.Request) {
	_, appId := ApiKeyFromContext(r.Context())
	input := createTicketInput{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(input.UserID) == 0 {
		http.Error(w, "empty user id", http.StatusBadRequest)
		return
	}
	if len(input.Topic) == 0 {
		http.Error(w, "empty topic", http.StatusBadRequest)
		return
	}

	auth, err := s.app.Auth(r.Context())
	if err != nil {
		s.logger.Error("error getting auth", "error", err)
		http.Error(w, "error getting auth", http.StatusInternalServerError)
		return
	}

	user, err := auth.GetUser(r.Context(), input.UserID)
	if err != nil && !errorutils.IsNotFound(err) {
		s.logger.Error("error getting user", "error", err)
		http.Error(w, "error getting user", http.StatusInternalServerError)
		return
	}
	if user == nil || errorutils.IsNotFound(err) {
		s.logger.Error("user not found", "userId", input.UserID, "appId", appId)
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	customClaims := make(map[string]any)
	customClaims[wsTokenAppIdClaimKey] = appId
	customClaims[wsTokenTopicClaimKey] = input.Topic
	customToken, err := auth.CustomTokenWithClaims(r.Context(), user.UID, customClaims)

	response := createTicketResponse{
		Token: customToken,
	}
	jsonResponse(w, http.StatusOK, response)
}

type broadcastInput struct {
	Payload map[string]any `json:"Payload"`
}

func (s *server) handleApiBroadcast(w http.ResponseWriter, r *http.Request) {
	_, appId := ApiKeyFromContext(r.Context())
	topicName := chi.URLParam(r, "topic")

	input := &broadcastInput{}
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to decode input: %v", err.Error()), http.StatusBadRequest)
		return
	}

	topic := s.wsTopicCollection.getTopic(appId, topicName)
	if topic == nil {
		http.Error(w, "topic not found", http.StatusInternalServerError)
		return
	}
	payloadBytes, err := json.Marshal(input.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	topic.Broker.Notifier <- []byte(payloadBytes)
	w.WriteHeader(http.StatusNoContent)
}
