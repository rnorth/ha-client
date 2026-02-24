package client

import (
	"encoding/json"
	"time"
)

type HAInfo struct {
	Version      string     `json:"version"`
	LocationName string     `json:"location_name"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Timezone     string     `json:"time_zone"`
	UnitSystem   UnitSystem `json:"unit_system"`
}

type UnitSystem struct {
	Length      string `json:"length"`
	Temperature string `json:"temperature"`
}

type State struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged time.Time              `json:"last_changed"`
	LastUpdated time.Time              `json:"last_updated"`
}

type ActionDomain struct {
	Domain   string                  `json:"domain"`
	Services map[string]ActionDetail `json:"services"`
}

type ActionDetail struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      map[string]interface{} `json:"fields"`
}

type Area struct {
	AreaID  string `json:"area_id" yaml:"area_id"`
	Name    string `json:"name" yaml:"name"`
	Picture string `json:"picture,omitempty" yaml:"picture,omitempty"`
}

type Device struct {
	ID            string   `json:"id" yaml:"id"`
	Name          string   `json:"name" yaml:"name"`
	AreaID        string   `json:"area_id,omitempty" yaml:"area_id,omitempty"`
	Manufacturer  string   `json:"manufacturer,omitempty" yaml:"manufacturer,omitempty"`
	Model         string   `json:"model,omitempty" yaml:"model,omitempty"`
	ConfigEntries []string `json:"config_entries,omitempty" yaml:"config_entries,omitempty"`
}

type EntityEntry struct {
	EntityID   string  `json:"entity_id" yaml:"entity_id"`
	Name       string  `json:"name,omitempty" yaml:"name,omitempty"`
	AreaID     string  `json:"area_id,omitempty" yaml:"area_id,omitempty"`
	DeviceID   string  `json:"device_id,omitempty" yaml:"device_id,omitempty"`
	Platform   string  `json:"platform,omitempty" yaml:"platform,omitempty"`
	// DisabledBy is nil when the entity is enabled. A non-nil value names the
	// source that disabled it (e.g. "user", "integration", "config_entry").
	DisabledBy *string `json:"disabled_by,omitempty" yaml:"disabled_by,omitempty"`
	UniqueID   string  `json:"unique_id,omitempty" yaml:"unique_id,omitempty"`
}

type WSMessage struct {
	ID      int             `json:"id,omitempty"`
	Type    string          `json:"type"`
	Success bool            `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
	Error   *WSError        `json:"error,omitempty"`
}

type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
