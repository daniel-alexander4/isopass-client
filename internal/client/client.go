package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	serverURL   string
	bearerToken string
	httpClient  *http.Client

	mu              sync.Mutex
	shortToken      string
	shortTokenExpAt time.Time
}

type ClientInfo struct {
	Email         string   `json:"email"`
	Organizations []string `json:"organizations"`
	Scopes        []string `json:"scopes"`
}

type Secret struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	OrganizationID string   `json:"organization_id"`
	Organization   string   `json:"organization"`
	Tags           []string `json:"tags"`
	Scopes         []string `json:"scopes"`
	Value          string   `json:"value"`
}

type tokenExchangeResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type TLSOptions struct {
	SkipVerify bool
	CACertPath string
}

// HTTPClientFromConfig returns an *http.Client configured with TLS settings from the saved config.
// Falls back to http.DefaultClient if no config is found.
func HTTPClientFromConfig() *http.Client {
	cfg, err := LoadConfig()
	if err != nil {
		return &http.Client{Timeout: 30 * time.Second}
	}
	c := &http.Client{Timeout: 30 * time.Second}
	if cfg.TLSCACert != "" {
		caCert, err := os.ReadFile(cfg.TLSCACert)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caCert)
			c.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool},
			}
		}
	} else if cfg.TLSSkipVerify {
		c.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-configured for self-signed certs
		}
	}
	return c
}

func New(serverURL, bearerToken string, tlsOpts TLSOptions) (*Client, error) {
	if !strings.HasPrefix(serverURL, "https://") {
		return nil, fmt.Errorf("server URL must use https")
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if tlsOpts.CACertPath != "" {
		caCert, err := os.ReadFile(tlsOpts.CACertPath)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caCert)
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool},
			}
		}
	} else if tlsOpts.SkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-configured for self-signed certs
		}
	}
	return &Client{
		serverURL:   strings.TrimRight(serverURL, "/"),
		bearerToken: bearerToken,
		httpClient:  httpClient,
	}, nil
}

func NewFromConfig() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return New(cfg.ServerURL, cfg.BearerToken, TLSOptions{
		SkipVerify: cfg.TLSSkipVerify,
		CACertPath: cfg.TLSCACert,
	})
}

func (c *Client) exchangeToken() (string, error) {
	req, err := http.NewRequest("POST", c.serverURL+"/api/auth/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}
	var data tokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	expAt, err := time.Parse(time.RFC3339, data.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("token exchange: bad expires_at: %w", err)
	}
	c.shortToken = data.Token
	c.shortTokenExpAt = expAt
	return c.shortToken, nil
}

func (c *Client) ensureShortToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shortToken != "" && time.Now().Before(c.shortTokenExpAt.Add(-30*time.Second)) {
		return c.shortToken, nil
	}
	return c.exchangeToken()
}

func (c *Client) clearShortToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shortToken = ""
	c.shortTokenExpAt = time.Time{}
}

// doAuthed performs an authenticated request with 401 retry (re-exchange token).
func (c *Client) doAuthed(method, path string, body any) (*http.Response, error) {
	do := func(token string) (*http.Response, error) {
		var bodyReader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(data)
		}
		req, err := http.NewRequest(method, c.serverURL+path, bodyReader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return c.httpClient.Do(req)
	}

	token, err := c.ensureShortToken()
	if err != nil {
		return nil, err
	}
	resp, err := do(token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.clearShortToken()
		token, err = c.ensureShortToken()
		if err != nil {
			return nil, err
		}
		resp, err = do(token)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (c *Client) Me() (*ClientInfo, error) {
	resp, err := c.doAuthed("GET", "/api/client/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client/me failed (%d): %s", resp.StatusCode, string(body))
	}
	var info ClientInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) Search(limit int) ([]Secret, error) {
	if limit <= 0 {
		limit = 200
	}
	body := map[string]int{"limit": limit}
	resp, err := c.doAuthed("POST", "/api/client/search", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (%d): %s", resp.StatusCode, string(b))
	}
	var secrets []Secret
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}
