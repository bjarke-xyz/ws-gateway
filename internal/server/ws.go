package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"firebase.google.com/go/v4/auth"
	"github.com/gorilla/websocket"
)

type WsTopicCollection struct {
	Topics  map[TopicID]*WsTopic
	Cancels map[TopicID]context.CancelFunc
	*sync.RWMutex
}

func (tc *WsTopicCollection) getTopic(appId string, topic string) *WsTopic {
	topicId := CreateTopicID(appId, topic)
	tc.Lock()
	defer tc.Unlock()
	tp, ok := tc.Topics[topicId]
	if !ok {
		return nil
	}
	return tp
}

func (tc *WsTopicCollection) createTopicIfNotExists(appId string, topic string, logger *slog.Logger) *WsTopic {
	topicId := CreateTopicID(appId, topic)
	tc.Lock()
	defer tc.Unlock()
	tp, ok := tc.Topics[topicId]
	if ok {
		return tp
	} else {
		broker := &WsBroker{
			Notifier:       make(chan []byte, 1),
			newClients:     make(chan chan []byte),
			closingClients: make(chan chan []byte),
			clients:        make(map[chan []byte]bool),
			RWMutex:        &sync.RWMutex{},
		}
		ctx, cancel := context.WithCancel(context.Background())
		tp = &WsTopic{
			Clients:         make(map[ClientID]*WsClient),
			Topic:           topic,
			ID:              topicId,
			Broker:          broker,
			TopicCollection: tc,
			ctx:             ctx,
			RWMutex:         &sync.RWMutex{},
		}
		tc.Topics[topicId] = tp
		tc.Cancels[topicId] = cancel
		go tp.Listen(logger)
		return tp
	}
}

func (tc *WsTopicCollection) deleteTopic(topicId TopicID) {
	tc.Lock()
	defer tc.Unlock()
	cancel, ok := tc.Cancels[topicId]
	if !ok {
		return
	}
	cancel()
	delete(tc.Topics, topicId)
	delete(tc.Cancels, topicId)
}

type WsTopic struct {
	Clients         map[ClientID]*WsClient
	Topic           string
	ID              TopicID
	Broker          *WsBroker
	TopicCollection *WsTopicCollection
	ctx             context.Context
	*sync.RWMutex
}

func (tp *WsTopic) add(client *WsClient) {
	tp.Lock()
	defer tp.Unlock()
	tp.Clients[client.ID] = client
}

func (tp *WsTopic) del(clientId ClientID) {
	tp.Lock()
	defer tp.Unlock()
	delete(tp.Clients, clientId)
	if len(tp.Clients) == 0 {
		tp.TopicCollection.deleteTopic(tp.ID)
	}
}

type TopicID string

func CreateTopicID(appId string, topic string) TopicID {
	return TopicID(appId + ":" + topic)
}

type ClientID string // UUID

type WsClient struct {
	Conn  *websocket.Conn
	ID    ClientID
	Token *auth.Token
	Topic *WsTopic
}

func (c *WsClient) appId() string {
	return getClaim(c.Token, wsTokenAppIdClaimKey)
}
func (c *WsClient) topic() string {
	return getClaim(c.Token, wsTokenTopicClaimKey)
}

type WsBroker struct {
	// Events are pushed to this channel by the main events-gathering routine
	Notifier chan []byte

	// New client connections
	newClients chan chan []byte

	// Closed client connections
	closingClients chan chan []byte

	// Client connections registry
	clients map[chan []byte]bool

	*sync.RWMutex
}

func (b *WsBroker) registerClient(s chan []byte) {
	b.Lock()
	defer b.Unlock()
	b.clients[s] = true
}
func (b *WsBroker) delClient(s chan []byte) {
	b.Lock()
	defer b.Unlock()
	delete(b.clients, s)
}

func (tp *WsTopic) Listen(logger *slog.Logger) {
	for {
		select {
		case <-tp.ctx.Done():
			logger.Info("ctx.Done called in broker", "clients", len(tp.Broker.clients))
			return
		case s := <-tp.Broker.newClients:
			// A new client has connected.
			// Register their message channel
			tp.Broker.registerClient(s)
			logger.Info("Client added", "clients", len(tp.Broker.clients))
		case s := <-tp.Broker.closingClients:
			// A client has dettached and we want to
			// stop sending them messages.
			tp.Broker.delClient(s)
			logger.Info("Removed client", "clients", len(tp.Broker.clients))
		case event := <-tp.Broker.Notifier:
			// We got a new event from the outside!
			// Send event to all connected clients
			for clientMessageChan := range tp.Broker.clients {
				clientMessageChan <- event
			}
		}
	}

}

const (
	wsNameHeader = "WS-NAME"
	wsIdHeader   = "WS-ID"
)

const (
	readBuffSize = 2 << 10
	writeBuffSize
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  readBuffSize,
	WriteBufferSize: writeBuffSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
