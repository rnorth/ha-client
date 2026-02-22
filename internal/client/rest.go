package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RESTClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewRESTClient(serverURL, token string) (*RESTClient, error) {
	url := strings.TrimRight(serverURL, "/")
	return &RESTClient{
		baseURL: url,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *RESTClient) post(path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
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

func (c *RESTClient) CallAction(domain, action string, data map[string]interface{}) error {
	return c.post("/api/services/"+domain+"/"+action, data, nil)
}
