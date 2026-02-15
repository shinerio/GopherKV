package client

import (
	"bytes"
	"context"
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
	baseURL string
	http    *http.Client
}

func New(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl int64) error {
	body := map[string]interface{}{
		"key":   key,
		"value": base64.StdEncoding.EncodeToString(value),
		"ttl":   ttl,
	}
	_, err := c.do(ctx, http.MethodPut, "/v1/key", body)
	return err
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, int64, error) {
	query := "/v1/key?k=" + url.QueryEscape(key)
	resp, err := c.do(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, 0, err
	}
	var data struct {
		Value string `json:"value"`
		TTL   int64  `json:"ttl_remaining"`
	}
	if err := mapToStruct(resp.Data, &data); err != nil {
		return nil, 0, err
	}
	val, err := base64.StdEncoding.DecodeString(data.Value)
	if err != nil {
		return nil, 0, err
	}
	return val, data.TTL, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v1/key?k="+url.QueryEscape(key), nil)
	return err
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/exists?k="+url.QueryEscape(key), nil)
	if err != nil {
		return false, err
	}
	var data struct {
		Exists bool `json:"exists"`
	}
	if err := mapToStruct(resp.Data, &data); err != nil {
		return false, err
	}
	return data.Exists, nil
}

func (c *Client) TTL(ctx context.Context, key string) (int64, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/ttl?k="+url.QueryEscape(key), nil)
	if err != nil {
		return 0, err
	}
	var data struct {
		TTL int64 `json:"ttl"`
	}
	if err := mapToStruct(resp.Data, &data); err != nil {
		return 0, err
	}
	return data.TTL, nil
}

func (c *Client) Stats(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/stats", nil)
	if err != nil {
		return nil, err
	}
	if m, ok := resp.Data.(map[string]interface{}); ok {
		return m, nil
	}
	return nil, fmt.Errorf("invalid stats response")
}

func (c *Client) Snapshot(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.do(ctx, http.MethodPost, "/v1/snapshot", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if m, ok := resp.Data.(map[string]interface{}); ok {
		return m, nil
	}
	return nil, fmt.Errorf("invalid snapshot response")
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (protocol.APIResponse, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return protocol.APIResponse{}, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return protocol.APIResponse{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return protocol.APIResponse{}, err
	}
	defer resp.Body.Close()
	var apiResp protocol.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return protocol.APIResponse{}, err
	}
	if apiResp.Code != protocol.CodeOK {
		return apiResp, protocol.NewError(apiResp.Code, apiResp.Msg)
	}
	return apiResp, nil
}

func mapToStruct(v interface{}, target interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
