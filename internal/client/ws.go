package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	counter atomic.Int32
}

func NewWSClient(serverURL, token string) (*WSClient, error) {
	// Trim trailing slash so we never produce "//api/websocket".
	wsURL := strings.TrimRight(serverURL, "/")
	// Convert http:// → ws://, https:// → wss://
	if strings.HasPrefix(wsURL, "http:") {
		wsURL = "ws:" + wsURL[5:]
	} else if strings.HasPrefix(wsURL, "https:") {
		wsURL = "wss:" + wsURL[6:]
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/api/websocket", nil)
	if err != nil {
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	// Auth flow
	var authRequired WSMessage
	if err := conn.ReadJSON(&authRequired); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read auth_required: %w", err)
	}

	if err := conn.WriteJSON(map[string]string{"type": "auth", "access_token": token}); err != nil {
		conn.Close()
		return nil, err
	}

	var authResult WSMessage
	if err := conn.ReadJSON(&authResult); err != nil {
		conn.Close()
		return nil, err
	}
	if authResult.Type != "auth_ok" {
		conn.Close()
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	return &WSClient{conn: conn}, nil
}

func (c *WSClient) Close() error {
	return c.conn.Close()
}

func (c *WSClient) send(msgType string, extra map[string]interface{}) (*WSMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := int(c.counter.Add(1))
	msg := map[string]interface{}{"type": msgType}
	for k, v := range extra {
		msg[k] = v
	}
	msg["id"] = id // set AFTER merge so it cannot be overwritten by config fields

	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("send %s: %w", msgType, err)
	}

	var resp WSMessage
	if err := c.conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("read response for %s: %w", msgType, err)
	}
	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("WS error %s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("command %s failed", msgType)
	}
	return &resp, nil
}

func (c *WSClient) ListAreas() ([]Area, error) {
	resp, err := c.send("config/area_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var areas []Area
	return areas, json.Unmarshal(resp.Result, &areas)
}

func (c *WSClient) CreateArea(name string) (*Area, error) {
	resp, err := c.send("config/area_registry/create", map[string]interface{}{"name": name})
	if err != nil {
		return nil, err
	}
	var area Area
	return &area, json.Unmarshal(resp.Result, &area)
}

func (c *WSClient) DeleteArea(areaID string) error {
	_, err := c.send("config/area_registry/delete", map[string]interface{}{"area_id": areaID})
	return err
}

func (c *WSClient) ListDevices() ([]Device, error) {
	resp, err := c.send("config/device_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var devices []Device
	return devices, json.Unmarshal(resp.Result, &devices)
}

func (c *WSClient) ListEntities() ([]EntityEntry, error) {
	resp, err := c.send("config/entity_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var entities []EntityEntry
	return entities, json.Unmarshal(resp.Result, &entities)
}

func (c *WSClient) GetEntity(entityID string) (*EntityEntry, error) {
	resp, err := c.send("config/entity_registry/get", map[string]interface{}{"entity_id": entityID})
	if err != nil {
		return nil, err
	}
	var entity EntityEntry
	return &entity, json.Unmarshal(resp.Result, &entity)
}

// GetAutomationConfig fetches the automation config for the given HA entity ID (e.g. "automation.my_automation"); it resolves the entity ID to the storage ID internally via the entity registry.
func (c *WSClient) GetAutomationConfig(entityID string) (map[string]interface{}, error) {
	resp, err := c.send("automation/config", map[string]interface{}{"entity_id": entityID})
	if err != nil {
		return nil, err
	}
	// HA wraps the automation config in a "config" key: {"config": {...}}
	var wrapper struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(resp.Result, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Config != nil {
		return wrapper.Config, nil
	}
	// Fallback: return the raw result if no wrapper
	var cfg map[string]interface{}
	return cfg, json.Unmarshal(resp.Result, &cfg)
}

// SubscribeEvents subscribes to events and calls handler for each event received.
// Blocks until handler returns false or an error occurs.
func (c *WSClient) SubscribeEvents(eventType string, handler func(json.RawMessage) bool) error {
	c.mu.Lock()
	id := int(c.counter.Add(1))
	msg := map[string]interface{}{"id": id, "type": "subscribe_events"}
	if eventType != "" {
		msg["event_type"] = eventType
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		c.mu.Unlock()
		return err
	}

	// Read and validate subscription confirmation.
	var ack WSMessage
	if err := c.conn.ReadJSON(&ack); err != nil {
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()
	if !ack.Success {
		if ack.Error != nil {
			return fmt.Errorf("subscribe failed: %s: %s", ack.Error.Code, ack.Error.Message)
		}
		return fmt.Errorf("subscribe_events failed")
	}

	// Stream events
	for {
		var event WSMessage
		if err := c.conn.ReadJSON(&event); err != nil {
			return err
		}
		if event.Type == "event" {
			if !handler(event.Event) {
				return nil
			}
		}
	}
}
