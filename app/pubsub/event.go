package pubsub

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"

	d "github.com/denniswon/validationcloud/app/data"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// EventConsumer - Event consumption to be managed by this struct, when new websocket
// connection requests for receiving event data, it'll create this struct, with necessary pieces
// of information, which is to be required when delivering data & checking whether this connection
// has really requested notification for this event or not
type EventConsumer struct {
	Client     *redis.Client
	Requests   map[string]*SubscriptionRequest
	Connection *websocket.Conn
	PubSub     *redis.PubSub
	DB         *gorm.DB
	ConnLock   *sync.Mutex
	TopicLock  *sync.RWMutex
}

// Subscribe - Event consumer is subscribing to `event` topic,
// where all event related data to be published
func (e *EventConsumer) Subscribe() {
	e.PubSub = e.Client.Subscribe(context.Background(), "event")
}

// Listen - Polling for new data published in `event` topic periodically
// and sending data to subscribed to client ( connected over websocket )
// if client has subscribed to get notified on occurrence of this event
func (e *EventConsumer) Listen() {

	for {

		msg, err := e.PubSub.ReceiveTimeout(context.Background(), time.Second)
		if err != nil {
			continue
		}

		switch m := msg.(type) {

		case *redis.Subscription:

			// Pubsub broker informed we've been unsubscribed from
			// this topic
			if m.Kind == "unsubscribe" {
				return
			}

			e.SendData(&SubscriptionResponse{
				Code:    1,
				Message: "Subscribed to `event`",
			})

		case *redis.Message:
			e.Send(m.Payload)

		}

	}

}

// Send - Sending event occurrence data to client application, which has subscribed to this event
// & connected over websocket
func (e *EventConsumer) Send(msg string) {

	var event struct {
		Origin          string         `json:"origin"`
		Index           uint           `json:"index"`
		Topics          pq.StringArray `json:"topics"`
		Data            string         `json:"data"`
		TransactionHash string         `json:"txHash"`
		BlockHash       string         `json:"blockHash"`
	}

	_msg := []byte(msg)

	if err := json.Unmarshal(_msg, &event); err != nil {
		log.Printf("[!] Failed to decode published event data to JSON : %s\n", err.Error())
		return
	}

	data := make([]byte, 0)
	var err error

	if len(event.Data) != 0 {
		data, err = hex.DecodeString(event.Data[2:])
	}

	if err != nil {
		log.Printf("[!] Failed to decode data field of event : %s\n", err.Error())
		return
	}

	_event := &d.Event{
		Origin:          event.Origin,
		Index:           event.Index,
		Topics:          event.Topics,
		Data:            data,
		TransactionHash: event.TransactionHash,
		BlockHash:       event.BlockHash,
	}

	var request *SubscriptionRequest

	// -- Obtaining read lock
	e.TopicLock.RLock()

	for _, v := range e.Requests {

		if v.DoesMatchWithPublishedEventData(_event) {
			request = v
			break
		}

	}

	e.TopicLock.RUnlock()
	// -- Unlocking shared resource

	// Can't proceed with this anymore, because failed to find
	// respective subscription request
	if request == nil {
		return
	}

	e.SendData(&event)

}

// SendData - Sending message to client application, connected over websocket
//
// If failed, we're going to remove subscription & close websocket
// connection ( connection might be already closed though )
func (e *EventConsumer) SendData(data interface{}) bool {

	// -- Critical section of code begins
	//
	// Attempting to write to a network resource,
	// shared among multiple go routines
	e.ConnLock.Lock()
	defer e.ConnLock.Unlock()

	if err := e.Connection.WriteJSON(data); err != nil {
		log.Printf("[!] Failed to deliver `event` data to client : %s\n", err.Error())
		return false
	}

	return true

}

// Unsubscribe - Unsubscribe from event data publishing topic, to be called
// when stopping to listen data being published on this pubsub channel
// due to client has requested a unsubscription/ network connection got hampered
func (e *EventConsumer) Unsubscribe() {

	if e.PubSub == nil {
		log.Printf("[!] Bad attempt to unsubscribe from `event` topic\n")
		return
	}

	if err := e.PubSub.Unsubscribe(context.Background(), "event"); err != nil {
		log.Printf("[!] Failed to unsubscribe from `event` topic : %s\n", err.Error())
		return
	}

	resp := &SubscriptionResponse{
		Code:    1,
		Message: "Unsubscribed from `event`",
	}

	// -- Critical section of code begins
	//
	// Attempting to write to a network resource,
	// shared among multiple go routines
	e.ConnLock.Lock()
	defer e.ConnLock.Unlock()

	if err := e.Connection.WriteJSON(resp); err != nil {

		log.Printf("[!] Failed to deliver `event` unsubscription confirmation to client : %s\n", err.Error())
		return

	}

}
