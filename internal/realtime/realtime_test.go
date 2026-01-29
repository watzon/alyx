package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()
	cfg := &config.DatabaseConfig{
		Path: ":memory:",
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func testSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Parse([]byte(`
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
      author_id:
        type: string
        index: true
      published:
        type: bool
        default: false
`))
	if err != nil {
		t.Fatalf("Failed to parse test schema: %v", err)
	}
	return s
}

func setupTestDB(t *testing.T, db *database.DB, s *schema.Schema) {
	t.Helper()
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, stmt)
		}
	}
}

func TestSubscriptionIndex(t *testing.T) {
	idx := NewSubscriptionIndex()

	sub1 := &Subscription{ID: "sub1", Collection: "posts"}
	sub2 := &Subscription{ID: "sub2", Collection: "posts"}
	sub3 := &Subscription{ID: "sub3", Collection: "users"}

	idx.Add(sub1)
	idx.Add(sub2)
	idx.Add(sub3)

	if idx.Count() != 3 {
		t.Errorf("Expected count 3, got %d", idx.Count())
	}

	if idx.CollectionCount("posts") != 2 {
		t.Errorf("Expected posts count 2, got %d", idx.CollectionCount("posts"))
	}

	candidates := idx.GetCandidates("posts")
	if len(candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(candidates))
	}

	idx.Remove(sub1)
	if idx.CollectionCount("posts") != 1 {
		t.Errorf("Expected posts count 1 after removal, got %d", idx.CollectionCount("posts"))
	}
}

func TestSubscriptionMatching(t *testing.T) {
	sub := &Subscription{
		ID:         "sub1",
		Collection: "posts",
	}

	change1 := &Change{Collection: "posts", Operation: OperationInsert}
	change2 := &Change{Collection: "users", Operation: OperationInsert}

	if !sub.MatchesChange(change1) {
		t.Error("Expected subscription to match posts change")
	}

	if sub.MatchesChange(change2) {
		t.Error("Expected subscription to not match users change")
	}
}

func TestNewSubscription(t *testing.T) {
	payload := &SubscribePayload{
		Collection: "posts",
		Filter: map[string]Filter{
			"published": {Eq: true},
		},
		Sort:  []string{"-created_at"},
		Limit: 50,
	}

	sub := NewSubscription("client1", payload, nil)

	if sub.Collection != "posts" {
		t.Errorf("Expected collection posts, got %s", sub.Collection)
	}

	if sub.Limit != 50 {
		t.Errorf("Expected limit 50, got %d", sub.Limit)
	}

	if sub.State != SubscriptionStateActive {
		t.Errorf("Expected state active, got %s", sub.State)
	}
}

func TestNewSubscriptionLimitCapping(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{"zero defaults to 100", 0, 100},
		{"negative defaults to 100", -1, 100},
		{"under max stays same", 500, 500},
		{"over max capped to 1000", 2000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &SubscribePayload{
				Collection: "posts",
				Limit:      tt.inputLimit,
			}
			sub := NewSubscription("client1", payload, nil)
			if sub.Limit != tt.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tt.expectedLimit, sub.Limit)
			}
		})
	}
}

func TestChangesIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		changes  Changes
		expected bool
	}{
		{"empty", Changes{}, true},
		{"with insert", Changes{Inserts: []any{map[string]any{"id": "1"}}}, false},
		{"with update", Changes{Updates: []any{map[string]any{"id": "1"}}}, false},
		{"with delete", Changes{Deletes: []string{"1"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.changes.IsEmpty() != tt.expected {
				t.Errorf("Expected IsEmpty() = %v, got %v", tt.expected, tt.changes.IsEmpty())
			}
		})
	}
}

func TestBrokerBasic(t *testing.T) {
	db := testDB(t)
	s := testSchema(t)
	setupTestDB(t, db, s)

	cfg := &BrokerConfig{
		PollInterval:   100,
		MaxConnections: 100,
		BufferSize:     100,
	}

	broker := NewBroker(db, s, nil, cfg)
	if broker == nil {
		t.Fatal("Failed to create broker")
	}

	if broker.ClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", broker.ClientCount())
	}

	if broker.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", broker.SubscriptionCount())
	}
}

func TestMessageTypes(t *testing.T) {
	tests := []struct {
		msgType  MessageType
		expected string
	}{
		{MessageTypeSubscribe, "subscribe"},
		{MessageTypeUnsubscribe, "unsubscribe"},
		{MessageTypePing, "ping"},
		{MessageTypeConnected, "connected"},
		{MessageTypeSnapshot, "snapshot"},
		{MessageTypeDelta, "delta"},
		{MessageTypeError, "error"},
		{MessageTypePong, "pong"},
	}

	for _, tt := range tests {
		if string(tt.msgType) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.msgType)
		}
	}
}

func TestOperationTypes(t *testing.T) {
	tests := []struct {
		op       Operation
		expected string
	}{
		{OperationInsert, "INSERT"},
		{OperationUpdate, "UPDATE"},
		{OperationDelete, "DELETE"},
	}

	for _, tt := range tests {
		if string(tt.op) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.op)
		}
	}
}

func TestMessageJSON(t *testing.T) {
	msg := Message{
		ID:   "msg1",
		Type: MessageTypeSubscribe,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if decoded.ID != msg.ID {
		t.Errorf("Expected ID %s, got %s", msg.ID, decoded.ID)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Expected type %s, got %s", msg.Type, decoded.Type)
	}
}

func TestWebSocketHandshake(t *testing.T) {
	db := testDB(t)
	s := testSchema(t)
	setupTestDB(t, db, s)

	cfg := &BrokerConfig{
		PollInterval:   100,
		MaxConnections: 100,
		BufferSize:     100,
	}

	broker := NewBroker(db, s, nil, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker.Start(ctx)
	defer broker.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			t.Errorf("Failed to accept WebSocket: %v", err)
			return
		}

		client := NewClient(conn, broker)
		broker.RegisterClient(client)

		connPayload, _ := json.Marshal(&ConnectedPayload{ClientID: client.ID})

		payload := Message{
			Type:    MessageTypeConnected,
			Payload: connPayload,
		}
		data, _ := json.Marshal(payload)

		writeCtx, writeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer writeCancel()
		if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
			t.Errorf("Failed to write message: %v", err)
			return
		}

		time.Sleep(200 * time.Millisecond)
		broker.UnregisterClient(client.ID)
		conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, data, err := conn.Read(ctx2)
	if err != nil {
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			t.Skip("Connection closed normally before message received")
		}
		t.Fatalf("Failed to read message: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeConnected {
		t.Errorf("Expected connected message, got %s", msg.Type)
	}
}
