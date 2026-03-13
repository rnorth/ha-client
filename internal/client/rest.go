package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotFound is returned when the requested resource does not exist (HTTP 404).
// Callers can test for it with errors.Is rather than matching error strings.
var ErrNotFound = errors.New("not found")

type RESTClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewRESTClient(serverURL, token string) *RESTClient {
	url := strings.TrimRight(serverURL, "/")
	return &RESTClient{
		baseURL: url,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *RESTClient) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *RESTClient) postRaw(path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (c *RESTClient) post(path string, body interface{}, out interface{}) error {
	raw, err := c.postRaw(path, body)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *RESTClient) GetInfo() (*HAInfo, error) {
	var info HAInfo
	return &info, c.get("/api/config", &info)
}

func (c *RESTClient) ListStates() ([]State, error) {
	var states []State
	return states, c.get("/api/states", &states)
}

func (c *RESTClient) GetState(entityID string) (*State, error) {
	var state State
	return &state, c.get("/api/states/"+entityID, &state)
}

func (c *RESTClient) SetState(entityID, state string, attributes map[string]interface{}) (*State, error) {
	body := map[string]interface{}{"state": state}
	if attributes != nil {
		body["attributes"] = attributes
	}
	var result State
	return &result, c.post("/api/states/"+entityID, body, &result)
}

func (c *RESTClient) ListActions() ([]ActionDomain, error) {
	var actions []ActionDomain
	return actions, c.get("/api/services", &actions)
}

func (c *RESTClient) CallAction(domain, action string, data map[string]interface{}, returnResponse bool) (*ActionResponse, error) {
	path := "/api/services/" + domain + "/" + action
	if returnResponse {
		path += "?return_response"
	}
	raw, err := c.postRaw(path, data)
	if err != nil {
		return nil, err
	}
	resp := &ActionResponse{}
	if returnResponse {
		if err := json.Unmarshal(raw, resp); err != nil {
			return nil, fmt.Errorf("parsing action response: %w", err)
		}
	} else {
		if err := json.Unmarshal(raw, &resp.ChangedStates); err != nil {
			return nil, fmt.Errorf("parsing changed states: %w", err)
		}
	}
	return resp, nil
}

// GetAutomationConfig fetches the automation config for the given storage ID (the "id" field in the automation YAML, e.g. "abc-123"), not the entity ID.
func (c *RESTClient) GetAutomationConfig(automationID string) (map[string]interface{}, error) {
	var cfg map[string]interface{}
	return cfg, c.get("/api/config/automation/config/"+automationID, &cfg)
}

func (c *RESTClient) SaveAutomationConfig(automationID string, cfg map[string]interface{}) error {
	return c.post("/api/config/automation/config/"+automationID, cfg, nil)
}

// RenderTemplate evaluates a Jinja template server-side via POST /api/template.
func (c *RESTClient) RenderTemplate(template string) (string, error) {
	body := map[string]string{"template": template}
	raw, err := c.postRaw("/api/template", body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
