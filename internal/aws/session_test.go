package aws

import (
	"context"
	"testing"
)

func TestNewSession(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}
	if session.Region() != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", session.Region())
	}
	if session.Profile() != "default" {
		t.Errorf("expected profile 'default', got '%s'", session.Profile())
	}
}

func TestSession_SetRegion(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	err = session.SetRegion(ctx, "eu-west-2")
	if err != nil {
		t.Fatalf("unexpected error setting region: %v", err)
	}
	if session.Region() != "eu-west-2" {
		t.Errorf("expected region 'eu-west-2', got '%s'", session.Region())
	}
}

func TestSession_SetProfile(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	err = session.SetProfile(ctx, "default")
	if err != nil {
		t.Fatalf("unexpected error setting profile: %v", err)
	}
	if session.Profile() != "default" {
		t.Errorf("expected profile 'default', got '%s'", session.Profile())
	}
}

func TestSession_Config(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-west-2")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	cfg := session.Config()
	if cfg.Region != "us-west-2" {
		t.Errorf("expected config region 'us-west-2', got '%s'", cfg.Region)
	}
}

func TestSession_Clients(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	if session.EC2Client() == nil {
		t.Error("expected non-nil EC2 client")
	}
	if session.ECRClient() == nil {
		t.Error("expected non-nil ECR client")
	}
	if session.CloudWatchClient() == nil {
		t.Error("expected non-nil CloudWatch client")
	}
}

func TestSession_Reload(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, "default", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	// Reload should not error
	err = session.Reload(ctx)
	if err != nil {
		t.Fatalf("unexpected error reloading session: %v", err)
	}

	// Clients should still be valid
	if session.EC2Client() == nil {
		t.Error("expected non-nil EC2 client after reload")
	}
}
