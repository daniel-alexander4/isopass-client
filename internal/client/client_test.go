package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFieldsUsernamePassword(t *testing.T) {
	val, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	fields := ParseFields("username_password", string(val))
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Label != "Username" || fields[0].Value != "admin" || fields[0].Secret {
		t.Errorf("field[0] = %+v", fields[0])
	}
	if fields[1].Label != "Password" || fields[1].Value != "s3cret" || !fields[1].Secret {
		t.Errorf("field[1] = %+v", fields[1])
	}
}

func TestParseFieldsKeyValuePair(t *testing.T) {
	val, _ := json.Marshal(map[string]string{"API_KEY": "abc123"})
	fields := ParseFields("key_value_pair", string(val))
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Label != "Key" || fields[0].Value != "API_KEY" || fields[0].Secret {
		t.Errorf("field[0] = %+v", fields[0])
	}
	if fields[1].Label != "Value" || fields[1].Value != "abc123" || !fields[1].Secret {
		t.Errorf("field[1] = %+v", fields[1])
	}
}

func TestParseFieldsTLSBundle(t *testing.T) {
	val, _ := json.Marshal(map[string]string{
		"certificate": "CERT",
		"private_key": "KEY",
		"ca_chain":    "CA",
	})
	fields := ParseFields("tls_bundle", string(val))
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0].Label != "Certificate" || fields[0].Secret {
		t.Errorf("field[0] = %+v", fields[0])
	}
	if fields[1].Label != "Private Key" || !fields[1].Secret {
		t.Errorf("field[1] = %+v", fields[1])
	}
	if fields[2].Label != "CA Chain" || fields[2].Secret {
		t.Errorf("field[2] = %+v", fields[2])
	}
}

func TestParseFieldsDefaultSingleField(t *testing.T) {
	fields := ParseFields("password", "mypassword")
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if !fields[0].Secret {
		t.Error("expected secret field")
	}
}

func TestParseFieldsInvalidJSON(t *testing.T) {
	fields := ParseFields("username_password", "not-json")
	if len(fields) != 1 {
		t.Fatalf("expected 1 fallback field, got %d", len(fields))
	}
	if fields[0].Value != "not-json" {
		t.Errorf("expected raw value, got %q", fields[0].Value)
	}
}

func TestFilterSecretsByType(t *testing.T) {
	secrets := []Secret{
		{Type: "password", Tags: []string{"prod"}},
		{Type: "api_key", Tags: []string{"prod"}},
		{Type: "password", Tags: []string{"dev"}},
	}
	result := FilterSecrets(secrets, "", "password")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterSecretsByTerms(t *testing.T) {
	secrets := []Secret{
		{Type: "password", Tags: []string{"prod", "db"}},
		{Type: "password", Tags: []string{"staging", "db"}},
		{Type: "password", Tags: []string{"prod", "cache"}},
	}
	result := FilterSecrets(secrets, "prod, db", "")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Tags[0] != "prod" || result[0].Tags[1] != "db" {
		t.Errorf("unexpected match: %+v", result[0])
	}
}

func TestFilterSecretsPartialMatch(t *testing.T) {
	secrets := []Secret{
		{Type: "password", Tags: []string{"production"}},
		{Type: "password", Tags: []string{"staging"}},
	}
	result := FilterSecrets(secrets, "prod", "")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
}

func TestFilterSecretsSearchableFields(t *testing.T) {
	val, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	secrets := []Secret{
		{Type: "username_password", Tags: []string{"prod"}, Value: string(val)},
		{Type: "password", Tags: []string{"prod"}, Value: "pw"},
	}
	result := FilterSecrets(secrets, "admin", "")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Type != "username_password" {
		t.Errorf("wrong match: %+v", result[0])
	}
}

func TestFilterSecretsNoTermsNoType(t *testing.T) {
	secrets := []Secret{
		{Type: "password", Tags: []string{"a"}},
		{Type: "api_key", Tags: []string{"b"}},
	}
	result := FilterSecrets(secrets, "", "")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{ServerURL: "https://example.com", BearerToken: "tok123"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file permissions.
	path := filepath.Join(tmp, ".config", "isopass", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected 0600, got %04o", perm)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ServerURL != cfg.ServerURL || loaded.BearerToken != cfg.BearerToken {
		t.Errorf("mismatch: %+v vs %+v", loaded, cfg)
	}
}

func TestConfigLoadMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for missing config")
	}
}
