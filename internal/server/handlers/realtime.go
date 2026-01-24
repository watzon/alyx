package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/realtime"
)

// RealtimeHandler handles WebSocket connections for real-time subscriptions.
type RealtimeHandler struct {
	broker *realtime.Broker
}

// NewRealtimeHandler creates a new realtime handler.
func NewRealtimeHandler(broker *realtime.Broker) *RealtimeHandler {
	return &RealtimeHandler{broker: broker}
}

// HandleWebSocket upgrades HTTP connections to WebSocket and manages the client lifecycle.
func (h *RealtimeHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to accept WebSocket connection")
		return
	}

	client := realtime.NewClient(conn, h.broker)
	h.broker.RegisterClient(client)

	connectedPayload, _ := json.Marshal(&realtime.ConnectedPayload{
		ClientID: client.ID,
	})

	client.Send(&realtime.Message{
		Type:    realtime.MessageTypeConnected,
		Payload: connectedPayload,
	})

	defer func() {
		h.broker.UnregisterClient(client.ID)
	}()

	client.Run()
}
