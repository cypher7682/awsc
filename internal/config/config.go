// Package config handles application configuration including AWS profiles and regions.
package config

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// AWSRegions contains all available AWS regions.
var AWSRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"af-south-1",
	"ap-east-1",
	"ap-south-1",
	"ap-south-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-southeast-3",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ca-central-1",
	"eu-central-1",
	"eu-central-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-south-1",
	"eu-south-2",
	"eu-north-1",
	"me-south-1",
	"me-central-1",
	"sa-east-1",
}

// AppConfig holds the runtime application configuration.
type AppConfig struct {
	Profile string
	Region  string
}

// NewAppConfig creates a new AppConfig with defaults from environment or flags.
func NewAppConfig(profile, region string) *AppConfig {
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
		if profile == "" {
			profile = "default"
		}
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
		}
		if region == "" {
			region = "us-east-1"
		}
	}
	return &AppConfig{
		Profile: profile,
		Region:  region,
	}
}

// SetRegion updates the active region.
func (c *AppConfig) SetRegion(region string) error {
	for _, r := range AWSRegions {
		if r == region {
			c.Region = region
			return nil
		}
	}
	return fmt.Errorf("invalid region: %s", region)
}

// SetProfile updates the active profile.
func (c *AppConfig) SetProfile(profile string) {
	c.Profile = profile
}

// LoadAWSConfig loads the AWS SDK configuration based on the current app config.
func (c *AppConfig) LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(c.Region),
	}
	if c.Profile != "" && c.Profile != "default" {
		opts = append(opts, config.WithSharedConfigProfile(c.Profile))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}
