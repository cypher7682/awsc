package config

import (
	"os"
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
