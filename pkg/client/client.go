package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shinerio/gopher-kv/pkg/protocol"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (*protocol.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result protocol.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Set(key string, value []byte, ttl time.Duration) error {
	req := protocol.SetRequest{
		Key:   key,
		Value: base64.StdEncoding.EncodeToString(value),
	}
	if ttl > 0 {
		req.TTL = int(ttl.Seconds())
	}

	resp, err := c.doRequest("PUT", "/v1/key", req)
	if err != nil {
		return err
	}
	if resp.Code != protocol.CodeSuccess {
		return fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) Get(key string) ([]byte, error) {
	resp, err := c.doRequest("GET", "/v1/key?k="+url.QueryEscape(key), nil)
	if err != nil {
		return nil, err
	}
	if resp.Code == protocol.CodeKeyNotFound {
		return nil, nil
	}
	if resp.Code != protocol.CodeSuccess {
		return nil, fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data")
	}

	valueStr, ok := dataMap["value"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid value in response")
	}

	value, err := base64.StdEncoding.DecodeString(valueStr)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *Client) Delete(key string) error {
	resp, err := c.doRequest("DELETE", "/v1/key?k="+url.QueryEscape(key), nil)
	if err != nil {
		return err
	}
	if resp.Code != protocol.CodeSuccess {
		return fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) Del(key string) error {
	return c.Delete(key)
}

func (c *Client) Exists(key string) (bool, error) {
	value, err := c.Get(key)
	if err != nil {
		return false, err
	}
	return value != nil, nil
}

func (c *Client) TTL(key string) (int, error) {
	resp, err := c.doRequest("GET", "/v1/ttl?k="+url.QueryEscape(key), nil)
	if err != nil {
		return 0, err
	}
	if resp.Code != protocol.CodeSuccess {
		return 0, fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response data")
	}

	ttl, ok := dataMap["ttl"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid ttl in response")
	}

	return int(ttl), nil
}

func (c *Client) Health() error {
	resp, err := c.doRequest("GET", "/v1/health", nil)
	if err != nil {
		return err
	}
	if resp.Code != protocol.CodeSuccess {
		return fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) Stats() (*protocol.StatsResponseData, error) {
	resp, err := c.doRequest("GET", "/v1/stats", nil)
	if err != nil {
		return nil, err
	}
	if resp.Code != protocol.CodeSuccess {
		return nil, fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, err
	}

	var stats protocol.StatsResponseData
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *Client) Snapshot() (*protocol.SnapshotResponseData, error) {
	resp, err := c.doRequest("POST", "/v1/snapshot", nil)
	if err != nil {
		return nil, err
	}
	if resp.Code != protocol.CodeSuccess {
		return nil, fmt.Errorf("server error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, err
	}
	var snapshot protocol.SnapshotResponseData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
