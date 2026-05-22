package aws

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	// This test verifies client creation with default profile.
	// It will use whatever credentials are available (or none in CI).
	ctx := context.Background()
	client, err := NewClient(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client.Region() != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", client.Region())
	}
	if client.Profile() != "default" {
		t.Errorf("expected profile 'default', got '%s'", client.Profile())
	}
}

func TestClient_SetRegion(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	err = client.SetRegion(ctx, "eu-west-2")
	if err != nil {
		t.Fatalf("unexpected error setting region: %v", err)
	}
	if client.Region() != "eu-west-2" {
		t.Errorf("expected region 'eu-west-2', got '%s'", client.Region())
	}
}

func TestClient_SetProfile(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	// Setting profile to "default" should work without error
	err = client.SetProfile(ctx, "default")
	if err != nil {
		t.Fatalf("unexpected error setting profile: %v", err)
	}
	if client.Profile() != "default" {
		t.Errorf("expected profile 'default', got '%s'", client.Profile())
	}
}

func TestClient_Config(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "default", "us-west-2")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	cfg := client.Config()
	if cfg.Region != "us-west-2" {
		t.Errorf("expected config region 'us-west-2', got '%s'", cfg.Region)
	}
}
