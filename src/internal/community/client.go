// Package community is the app's client for the community Worker: listing
// and downloading approved scripts, registering, logging in and uploading.
package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Script is an approved script as the community lists it.
type Script struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Author      string `json:"author_name"`
	Downloads   int    `json:"downloads"`
	CreatedAt   int64  `json:"created_at"`
}

// Client talks to one community Worker.
type Client struct {
	BaseURL string // for example https://scriptboard-community.example.workers.dev
	HTTP    *http.Client
}

// New returns a client for baseURL. A trailing slash is dropped.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether a community address is set.
func (c *Client) Configured() bool { return c.BaseURL != "" }

// List returns the newest approved scripts.
func (c *Client) List(ctx context.Context, limit int) ([]Script, error) {
	var out struct {
		Scripts []Script `json:"scripts"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/scripts?limit=%d", limit), "", nil, &out); err != nil {
		return nil, err
	}
	return out.Scripts, nil
}

// Download returns the JSON of an approved script, in the form the app
// imports. It counts as one download on the community.
func (c *Client) Download(ctx context.Context, id string) (string, error) {
	raw, err := c.raw(ctx, http.MethodGet, "/api/scripts/"+url.PathEscape(id)+"/download", "", nil)
	return string(raw), err
}

// Register creates an account and returns its login token.
func (c *Client) Register(ctx context.Context, username, password string) (string, error) {
	return c.auth(ctx, "/api/auth/register", username, password)
}

// Login returns the login token of an account.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	return c.auth(ctx, "/api/auth/login", username, password)
}

// Logout ends the session of token.
func (c *Client) Logout(ctx context.Context, token string) error {
	return c.do(ctx, http.MethodPost, "/api/auth/logout", token, struct{}{}, nil)
}

// Upload sends a script for review. It needs a login token. The script is
// not public until the admin approves it.
func (c *Client) Upload(ctx context.Context, token string, name, icon, description, source string) (id, message string, err error) {
	var out struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	body := map[string]string{"name": name, "icon": icon, "description": description, "source": source}
	if err := c.do(ctx, http.MethodPost, "/api/scripts", token, body, &out); err != nil {
		return "", "", err
	}
	return out.ID, out.Message, nil
}

func (c *Client) auth(ctx context.Context, path, username, password string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	body := map[string]string{"username": username, "password": password}
	if err := c.do(ctx, http.MethodPost, path, "", body, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("服务器没有返回登录凭证")
	}
	return out.Token, nil
}

// do sends one JSON request and decodes the reply into out, when out is set.
// A failed request returns the message the Worker gave.
func (c *Client) do(ctx context.Context, method, path, token string, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	raw, err := c.raw(ctx, method, path, token, payload)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *Client) raw(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	if !c.Configured() {
		return nil, errors.New("还没有设置社区地址")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连不上社区：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return nil, errors.New(e.Error)
		}
		return nil, fmt.Errorf("社区返回 %s", resp.Status)
	}
	return data, nil
}
