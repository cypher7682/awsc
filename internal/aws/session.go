// Package aws provides the core AWS session management for awsc.
// All services share a single Session which holds the live aws.Config.
// When profile or region changes, the session rebuilds SDK clients.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Session is the unified AWS session. All service layers use this to get
// their SDK clients. When profile/region changes, clients are rebuilt.
type Session struct {
	mu      sync.RWMutex
	cfg     aws.Config
	profile string
	region  string

	// Cached SDK clients — rebuilt on profile/region change
	ec2Client *awsec2.Client
	ecrClient *ecr.Client
	eksClient *eks.Client
	cwClient  *cloudwatch.Client
	smClient  *secretsmanager.Client
}

// NewSession creates a new AWS session with the given profile and region.
func NewSession(ctx context.Context, profile, region string) (*Session, error) {
	cfg, err := loadConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}

	s := &Session{
		cfg:     cfg,
		profile: profile,
		region:  region,
	}
	s.rebuildClients()
	return s, nil
}

// Config returns the current AWS SDK config (for direct use if needed).
func (s *Session) Config() aws.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Region returns the current region.
func (s *Session) Region() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.region
}

// Profile returns the current profile.
func (s *Session) Profile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profile
}

// EC2Client returns the shared EC2 SDK client.
func (s *Session) EC2Client() *awsec2.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ec2Client
}

// ECRClient returns the shared ECR SDK client.
func (s *Session) ECRClient() *ecr.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ecrClient
}

// CloudWatchClient returns the shared CloudWatch SDK client.
func (s *Session) CloudWatchClient() *cloudwatch.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cwClient
}

// EKSClient returns the shared EKS SDK client.
func (s *Session) EKSClient() *eks.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eksClient
}

// SecretsManagerClient returns the shared Secrets Manager SDK client.
func (s *Session) SecretsManagerClient() *secretsmanager.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.smClient
}

// SetRegion updates the region and rebuilds all clients.
func (s *Session) SetRegion(ctx context.Context, region string) error {
	cfg, err := loadConfig(ctx, s.profile, region)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.cfg = cfg
	s.region = region
	s.rebuildClients()
	s.mu.Unlock()
	return nil
}

// SetProfile updates the profile and rebuilds all clients.
func (s *Session) SetProfile(ctx context.Context, profile string) error {
	cfg, err := loadConfig(ctx, profile, s.region)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.cfg = cfg
	s.profile = profile
	s.rebuildClients()
	s.mu.Unlock()
	return nil
}

// Reload forces a fresh credentials load (e.g. after running login_cmd).
func (s *Session) Reload(ctx context.Context) error {
	cfg, err := loadConfig(ctx, s.profile, s.region)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.cfg = cfg
	s.rebuildClients()
	s.mu.Unlock()
	return nil
}

// rebuildClients recreates all SDK clients from the current config.
// Must be called with lock held.
func (s *Session) rebuildClients() {
	s.ec2Client = awsec2.NewFromConfig(s.cfg)
	s.ecrClient = ecr.NewFromConfig(s.cfg)
	s.eksClient = eks.NewFromConfig(s.cfg)
	s.cwClient = cloudwatch.NewFromConfig(s.cfg)
	s.smClient = secretsmanager.NewFromConfig(s.cfg)
}

// loadConfig loads AWS SDK config for a given profile and region.
func loadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}
