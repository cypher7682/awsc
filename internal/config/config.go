// Package config handles application configuration including AWS profiles and regions.
package config

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/yaml.v3"
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
	User    *UserConfig
}

// UserConfig represents the user's ~/.config/awsc/config.yaml settings.
type UserConfig struct {
	// LoginCmd is a shell command to run when credentials are expired/missing.
	// Supports placeholders: #PROFILE and #REGION which are substituted at runtime.
	// Example: "aws sso login --profile #PROFILE"
	LoginCmd string `yaml:"login_cmd"`

	// EC2ConnectCmd is a shell command to connect (shell) to an EC2 instance.
	// Supports placeholders: #PROFILE, #REGION, #INSTANCEID which are substituted at runtime.
	// Example: "aws ssm start-session --target #INSTANCEID --profile #PROFILE --region #REGION"
	EC2ConnectCmd string `yaml:"ec2_connect_cmd"`
}

// DefaultConfigPath returns the path to ~/.config/awsc/config.yaml.
func DefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "awsc", "config.yaml")
}

// LoadUserConfig reads and parses the user config file.
// Returns an empty UserConfig (not nil) if the file doesn't exist.
func LoadUserConfig() *UserConfig {
	path := DefaultConfigPath()
	if path == "" {
		return &UserConfig{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &UserConfig{}
	}

	var uc UserConfig
	if err := yaml.Unmarshal(data, &uc); err != nil {
		return &UserConfig{}
	}
	return &uc
}

// ResolveLoginCmd substitutes #PROFILE and #REGION placeholders in the login command.
func (uc *UserConfig) ResolveLoginCmd(profile, region string) string {
	if uc == nil || uc.LoginCmd == "" {
		return ""
	}
	cmd := uc.LoginCmd
	cmd = strings.ReplaceAll(cmd, "#PROFILE", profile)
	cmd = strings.ReplaceAll(cmd, "#REGION", region)
	return cmd
}

// HasLoginCmd returns true if a login command is configured.
func (uc *UserConfig) HasLoginCmd() bool {
	return uc != nil && uc.LoginCmd != ""
}

// ResolveEC2ConnectCmd substitutes #PROFILE, #REGION, and #INSTANCEID placeholders
// in the ec2_connect_cmd.
func (uc *UserConfig) ResolveEC2ConnectCmd(profile, region, instanceID string) string {
	if uc == nil || uc.EC2ConnectCmd == "" {
		return ""
	}
	cmd := uc.EC2ConnectCmd
	cmd = strings.ReplaceAll(cmd, "#PROFILE", profile)
	cmd = strings.ReplaceAll(cmd, "#REGION", region)
	cmd = strings.ReplaceAll(cmd, "#INSTANCEID", instanceID)
	return cmd
}

// HasEC2ConnectCmd returns true if an EC2 connect command is configured.
func (uc *UserConfig) HasEC2ConnectCmd() bool {
	return uc != nil && uc.EC2ConnectCmd != ""
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
		User:    LoadUserConfig(),
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

// LoadProfiles returns all AWS profile names from ~/.aws/credentials and ~/.aws/config.
func LoadProfiles() []string {
	seen := make(map[string]bool)
	var profiles []string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{"default"}
	}

	// Parse ~/.aws/credentials — sections are [profile_name]
	credPath := filepath.Join(homeDir, ".aws", "credentials")
	for _, name := range parseINISections(credPath, false) {
		if !seen[name] {
			seen[name] = true
			profiles = append(profiles, name)
		}
	}

	// Parse ~/.aws/config — sections are [profile profile_name] (except [default])
	configPath := filepath.Join(homeDir, ".aws", "config")
	for _, name := range parseINISections(configPath, true) {
		if !seen[name] {
			seen[name] = true
			profiles = append(profiles, name)
		}
	}

	if len(profiles) == 0 {
		return []string{"default"}
	}
	return profiles
}

// parseINISections extracts section names from an INI-style file.
// If stripProfilePrefix is true, it strips the "profile " prefix from section
// headers (as used in ~/.aws/config).
func parseINISections(path string, stripProfilePrefix bool) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var sections []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
			continue
		}
		name := line[1 : len(line)-1]
		if stripProfilePrefix {
			name = strings.TrimPrefix(name, "profile ")
		}
		name = strings.TrimSpace(name)
		if name != "" {
			sections = append(sections, name)
		}
	}
	return sections
}
