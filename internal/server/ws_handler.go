package server

import (
	"net/http"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (s *server) wsTopicHandler(client *WsClient, w http.ResponseWriter, r *http.Request) {
	defer client.Conn.Close()

	// Each connection registers its own message channel with the Broker's connections registry
	messageChan := make(chan []byte)
	topic := client.Topic
	topic.Broker.newClients <- messageChan
	// Remove this client from the map of connected clients
	// when this handler exits.
	defer func() {
		topic.Broker.closingClients <- messageChan
	}()

	go func() {
		for msgBytes := range messageChan {
			err := client.Conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				s.logger.Error("failed to write ws msg", "error", err)
				break
			}
		}
	}()

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			s.logger.Error("error reading ws msg", "error", err)
			break
		}
	}
}

func (s *server) wsClientMiddleware(next func(cl *WsClient, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		tokenStr := query.Get("token")

		auth, err := s.app.Auth(r.Context())
		if err != nil {
			s.logger.Error("error getting firebase auth", "error", err)
			http.Error(w, "failed to get firebase auth", http.StatusInternalServerError)
			return
		}

		customToken, err := s.authClient.SignInWithCustomToken(r.Context(), tokenStr)
		if err != nil {
			s.logger.Error("failed to sign in with custom token", "error", err)
			http.Error(w, "failed to sign in with custom token", http.StatusBadRequest)
			return
		}
		if customToken.Error != nil {
			s.logger.Error("custom token returned error", "error", customToken.Error)
			http.Error(w, "custom token returned error", http.StatusBadRequest)
			return
		}
		verifiedToken, err := auth.VerifyIDToken(r.Context(), customToken.IdToken)
		if err != nil {
			s.logger.Error("failed to verify ws token", "error", err)
			http.Error(w, "failed to verify token", http.StatusBadRequest)
			return
		}

		appIdClaim := getClaim(verifiedToken, wsTokenAppIdClaimKey)
		if appIdClaim == "" {
			s.logger.Error("missing appId claim")
			http.Error(w, "missing appId claim", http.StatusBadRequest)
			return
		}
		appId := chi.URLParam(r, "app-id")
		if appIdClaim != appId {
			s.logger.Error("invalid app id claim", "appId", appId, "appIdClaim", appIdClaim)
			http.Error(w, "invalid appId claim", http.StatusBadRequest)
			return
		}

		topicClaim := getClaim(verifiedToken, wsTokenTopicClaimKey)
		if topicClaim == "" {
			s.logger.Error("missing topic claim")
			http.Error(w, "missing topic claim", http.StatusBadRequest)
			return
		}
		topic := chi.URLParam(r, "topic")
		if topicClaim != topic {
			s.logger.Error("invalid topic claim", "topic", topic, "topicClaim", topicClaim)
			http.Error(w, "invalid topic claim", http.StatusBadRequest)
			return
		}

		clientId := uuid.NewString()
		h := http.Header{}
		h.Add(wsIdHeader, clientId)
		conn, err := upgrader.Upgrade(w, r, h)
		if err != nil {
			s.logger.Error("Error while upgrading connection", "error", err, "token", tokenStr)
			return
		}

		client := &WsClient{
			Conn:  conn,
			ID:    ClientID(clientId),
			Token: verifiedToken,
		}

		tp := s.wsTopicCollection.createTopicIfNotExists(appId, topic, s.logger)
		client.Topic = tp
		tp.add(client)
		defer tp.del(client.ID)
		next(client, w, r)
	}
}

func getClaim(token *auth.Token, key string) string {
	val, ok := token.Claims[key]
	if !ok {
		return ""
	}
	valStr, ok := val.(string)
	if !ok {
		return ""
	}
	return valStr
}
