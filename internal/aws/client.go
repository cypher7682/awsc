// Package aws provides the core AWS client management for awsc.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

// Client manages AWS SDK configuration and provides access to service clients.
type Client struct {
	mu      sync.RWMutex
	cfg     aws.Config
	profile string
	region  string
}

// NewClient creates a new AWS client with the given profile and region.
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:     cfg,
		profile: profile,
		region:  region,
	}, nil
}

// Config returns the current AWS SDK config.
func (c *Client) Config() aws.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

// Region returns the current region.
func (c *Client) Region() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.region
}

// Profile returns the current profile.
func (c *Client) Profile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profile
}

// SetRegion updates the region and reloads the AWS config.
func (c *Client) SetRegion(ctx context.Context, region string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if c.profile != "" && c.profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(c.profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return err
	}

	c.cfg = cfg
	c.region = region
	return nil
}

// SetProfile updates the profile and reloads the AWS config.
func (c *Client) SetProfile(ctx context.Context, profile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(c.region),
	}
	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return err
	}

	c.cfg = cfg
	c.profile = profile
	return nil
}
