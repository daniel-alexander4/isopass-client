package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"isopass-client/internal/client"
)

type App struct {
	ctx       context.Context
	apiClient *client.Client
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

type ConfigData struct {
	ServerURL     string `json:"server_url"`
	BearerToken   string `json:"bearer_token"`
	TLSSkipVerify bool   `json:"tls_skip_verify"`
	TLSCACert     string `json:"tls_ca_cert"`
}

func (a *App) LoadSettings() (*ConfigData, error) {
	cfg, err := client.LoadConfig()
	if err != nil {
		return nil, err
	}
	return &ConfigData{
		ServerURL:     cfg.ServerURL,
		BearerToken:   cfg.BearerToken,
		TLSSkipVerify: cfg.TLSSkipVerify,
		TLSCACert:     cfg.TLSCACert,
	}, nil
}

func (a *App) SaveSettings(serverURL, bearerToken string, tlsSkipVerify bool, tlsCACert string) error {
	cfg := &client.Config{
		ServerURL:     serverURL,
		BearerToken:   bearerToken,
		TLSSkipVerify: tlsSkipVerify,
		TLSCACert:     tlsCACert,
	}
	if err := client.SaveConfig(cfg); err != nil {
		return err
	}
	c, err := client.New(serverURL, bearerToken, client.TLSOptions{
		SkipVerify: tlsSkipVerify,
		CACertPath: tlsCACert,
	})
	if err != nil {
		return err
	}
	a.apiClient = c
	return nil
}

func (a *App) Connect(serverURL, bearerToken string, tlsSkipVerify bool, tlsCACert string) (*client.ClientInfo, error) {
	c, err := client.New(serverURL, bearerToken, client.TLSOptions{
		SkipVerify: tlsSkipVerify,
		CACertPath: tlsCACert,
	})
	if err != nil {
		return nil, err
	}
	a.apiClient = c
	return a.apiClient.Me()
}

func (a *App) Search(limit int) ([]client.Secret, error) {
	if a.apiClient == nil {
		return nil, nil
	}
	return a.apiClient.Search(limit)
}

func (a *App) CheckOIDCStatus(serverURL string) (bool, error) {
	if serverURL == "" {
		return false, nil
	}
	resp, err := http.Get(serverURL + "/api/auth/oidc/status")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return false, err
	}
	var result struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	return result.Enabled, nil
}

func (a *App) OIDCAuthorizeURL(serverURL string) string {
	return fmt.Sprintf("%s/api/auth/oidc/authorize", serverURL)
}
