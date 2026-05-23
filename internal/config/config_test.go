package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppConfig_Defaults(t *testing.T) {
	// Unset env vars for clean test
	os.Unsetenv("AWS_PROFILE")
	os.Unsetenv("AWS_REGION")
	os.Unsetenv("AWS_DEFAULT_REGION")

	cfg := NewAppConfig("", "")

	if cfg.Profile != "default" {
		t.Errorf("expected profile 'default', got '%s'", cfg.Profile)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", cfg.Region)
	}
}

func TestNewAppConfig_FromEnv(t *testing.T) {
	os.Setenv("AWS_PROFILE", "staging")
	os.Setenv("AWS_REGION", "eu-west-2")
	defer os.Unsetenv("AWS_PROFILE")
	defer os.Unsetenv("AWS_REGION")

	cfg := NewAppConfig("", "")

	if cfg.Profile != "staging" {
		t.Errorf("expected profile 'staging', got '%s'", cfg.Profile)
	}
	if cfg.Region != "eu-west-2" {
		t.Errorf("expected region 'eu-west-2', got '%s'", cfg.Region)
	}
}

func TestNewAppConfig_ExplicitOverridesEnv(t *testing.T) {
	os.Setenv("AWS_PROFILE", "staging")
	os.Setenv("AWS_REGION", "eu-west-2")
	defer os.Unsetenv("AWS_PROFILE")
	defer os.Unsetenv("AWS_REGION")

	cfg := NewAppConfig("production", "us-west-2")

	if cfg.Profile != "production" {
		t.Errorf("expected profile 'production', got '%s'", cfg.Profile)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got '%s'", cfg.Region)
	}
}

func TestSetRegion_Valid(t *testing.T) {
	cfg := NewAppConfig("", "")

	err := cfg.SetRegion("eu-west-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got '%s'", cfg.Region)
	}
}

func TestSetRegion_Invalid(t *testing.T) {
	cfg := NewAppConfig("", "")

	err := cfg.SetRegion("invalid-region-1")
	if err == nil {
		t.Error("expected error for invalid region, got nil")
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("region should not have changed, got '%s'", cfg.Region)
	}
}

func TestSetProfile(t *testing.T) {
	cfg := NewAppConfig("", "")

	cfg.SetProfile("production")
	if cfg.Profile != "production" {
		t.Errorf("expected profile 'production', got '%s'", cfg.Profile)
	}
}

func TestLoadProfiles_FromCredentials(t *testing.T) {
	// Create a temp dir with a fake credentials file
	tmpDir := t.TempDir()
	credContent := `[default]
aws_access_key_id = AKIA123
aws_secret_access_key = secret

[staging]
aws_access_key_id = AKIA456
aws_secret_access_key = secret2

[production]
aws_access_key_id = AKIA789
aws_secret_access_key = secret3
`
	credPath := filepath.Join(tmpDir, "credentials")
	if err := os.WriteFile(credPath, []byte(credContent), 0644); err != nil {
		t.Fatal(err)
	}

	sections := parseINISections(credPath, false)
	expected := []string{"default", "staging", "production"}
	if len(sections) != len(expected) {
		t.Fatalf("expected %d sections, got %d: %v", len(expected), len(sections), sections)
	}
	for i, s := range sections {
		if s != expected[i] {
			t.Errorf("section %d: expected '%s', got '%s'", i, expected[i], s)
		}
	}
}

func TestLoadProfiles_FromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[default]
region = us-east-1

[profile staging]
region = eu-west-1

[profile production]
region = us-west-2
sso_start_url = https://example.awsapps.com/start
`
	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	sections := parseINISections(configPath, true)
	expected := []string{"default", "staging", "production"}
	if len(sections) != len(expected) {
		t.Fatalf("expected %d sections, got %d: %v", len(expected), len(sections), sections)
	}
	for i, s := range sections {
		if s != expected[i] {
			t.Errorf("section %d: expected '%s', got '%s'", i, expected[i], s)
		}
	}
}
