package server

import (
	"log/slog"
	"net/http"
	"sync"

	"firebase.google.com/go/v4/auth"
	"github.com/gorilla/websocket"
)

type WsTopicCollection struct {
	Topics map[TopicID]*WsTopic
	Wg     *sync.WaitGroup // TODO: What is this used for?
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
		tp = &WsTopic{
			Clients: make(map[ClientID]*WsClient),
			Topic:   topic,
			ID:      topicId,
			Broker:  broker,
			RWMutex: &sync.RWMutex{},
		}
		tc.Topics[topicId] = tp
		// TODO: How to clean this up?
		go tp.Broker.Listen(logger)
		return tp
	}
}

type WsTopic struct {
	Clients map[ClientID]*WsClient
	Topic   string
	ID      TopicID
	Broker  *WsBroker
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

func (b *WsBroker) Listen(logger *slog.Logger) {
	for {
		select {
		case s := <-b.newClients:
			// A new client has connected.
			// Register their message channel
			b.registerClient(s)
			logger.Info("Client added", "clients", len(b.clients))
		case s := <-b.closingClients:
			// A client has dettached and we want to
			// stop sending them messages.
			b.delClient(s)
			logger.Info("Removed client", "clients", len(b.clients))
		case event := <-b.Notifier:
			// We got a new event from the outside!
			// Send event to all connected clients
			for clientMessageChan := range b.clients {
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
