// Package eks provides the EKS service layer for awsc.
package eks

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// Cluster represents a simplified EKS cluster.
type Cluster struct {
	Name             string
	ARN              string
	Status           string
	Version          string
	PlatformVersion  string
	Endpoint         string
	RoleARN          string
	CreatedAt        time.Time
	Tags             map[string]string

	// VPC/Networking
	VPCID                 string
	SubnetIDs             []string
	SecurityGroupIDs      []string
	ClusterSecurityGroup  string
	EndpointPublicAccess  bool
	EndpointPrivateAccess bool
	PublicAccessCidrs     []string

	// IP configuration
	ServiceIpv4Cidr string
	ServiceIpv6Cidr string
	IpFamily        string // ipv4, ipv6, or dual-stack

	// Logging
	LoggingEnabled []string // api, audit, authenticator, controllerManager, scheduler

	// Encryption
	EncryptionKeyARN  string
	EncryptionResources []string

	// OIDC / Identity
	OIDCIssuer string

	// Certificate Authority
	CertificateAuthorityData string

	// Access Config
	AuthenticationMode       string
	BootstrapClusterCreatorAdminPermissions bool

	// Upgrade Policy
	SupportType string // STANDARD or EXTENDED

	// Zonal Shift
	ZonalShiftEnabled bool

	// Compute Config (EKS Auto Mode)
	ComputeConfigEnabled bool
	NodePools            []string
	NodeRoleARN          string

	// Storage Config
	BlockStorageEnabled bool

	// Remote Network Config
	RemoteNodeNetworks []string
	RemotePodNetworks  []string

	// Outpost Config
	OutpostARNs       []string
	ControlPlaneInstanceType string
	ControlPlanePlacement    string
}

// NodeGroup represents a simplified EKS managed node group.
type NodeGroup struct {
	Name          string
	ClusterName   string
	ARN           string
	Status        string
	CapacityType  string // ON_DEMAND or SPOT
	InstanceTypes []string
	AmiType       string
	DiskSize      int32
	DesiredSize   int32
	MinSize       int32
	MaxSize       int32
	Subnets       []string
	CreatedAt     time.Time
	Tags          map[string]string
}

// FargateProfile represents a simplified EKS Fargate profile.
type FargateProfile struct {
	Name                 string
	ClusterName          string
	ARN                  string
	Status               string
	PodExecutionRoleARN  string
	Subnets              []string
	Selectors            []FargateSelector
	CreatedAt            time.Time
	Tags                 map[string]string
}

// FargateSelector represents a Fargate profile selector.
type FargateSelector struct {
	Namespace string
	Labels    map[string]string
}

// EKSAPI defines the interface for EKS operations (for testability).
type EKSAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
	ListFargateProfiles(ctx context.Context, params *eks.ListFargateProfilesInput, optFns ...func(*eks.Options)) (*eks.ListFargateProfilesOutput, error)
	DescribeFargateProfile(ctx context.Context, params *eks.DescribeFargateProfileInput, optFns ...func(*eks.Options)) (*eks.DescribeFargateProfileOutput, error)
}

// Service provides EKS operations.
type Service struct {
	client EKSAPI
}

// NewService creates a new EKS service from an AWS config.
func NewService(cfg aws.Config) *Service {
	return &Service{
		client: eks.NewFromConfig(cfg),
	}
}

// NewServiceFromClient creates a new EKS service from a pre-built SDK client.
func NewServiceFromClient(client *eks.Client) *Service {
	return &Service{client: client}
}

// NewServiceWithAPI creates a new EKS service with a custom API (for testing).
func NewServiceWithAPI(api EKSAPI) *Service {
	return &Service{client: api}
}

// ListClusters returns all EKS cluster names.
func (s *Service) ListClusters(ctx context.Context) ([]string, error) {
	var clusters []string
	var nextToken *string

	for {
		input := &eks.ListClustersInput{
			NextToken: nextToken,
		}
		output, err := s.client.ListClusters(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}

		clusters = append(clusters, output.Clusters...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return clusters, nil
}

// GetCluster returns detailed information about a cluster.
func (s *Service) GetCluster(ctx context.Context, name string) (*Cluster, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(name),
	}
	output, err := s.client.DescribeCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing cluster %s: %w", name, err)
	}
	if output.Cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", name)
	}
	return clusterFromAWS(output.Cluster), nil
}

// ListAllClusters returns detailed information about all clusters.
func (s *Service) ListAllClusters(ctx context.Context) ([]Cluster, error) {
	names, err := s.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	clusters := make([]Cluster, 0, len(names))
	for _, name := range names {
		cluster, err := s.GetCluster(ctx, name)
		if err != nil {
			// Log but continue - cluster might have been deleted
			continue
		}
		clusters = append(clusters, *cluster)
	}
	return clusters, nil
}

// ListNodeGroups returns all node group names for a cluster.
func (s *Service) ListNodeGroups(ctx context.Context, clusterName string) ([]string, error) {
	var nodeGroups []string
	var nextToken *string

	for {
		input := &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
			NextToken:   nextToken,
		}
		output, err := s.client.ListNodegroups(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("listing node groups for %s: %w", clusterName, err)
		}

		nodeGroups = append(nodeGroups, output.Nodegroups...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return nodeGroups, nil
}

// GetNodeGroup returns detailed information about a node group.
func (s *Service) GetNodeGroup(ctx context.Context, clusterName, nodeGroupName string) (*NodeGroup, error) {
	input := &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
	}
	output, err := s.client.DescribeNodegroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing node group %s/%s: %w", clusterName, nodeGroupName, err)
	}
	if output.Nodegroup == nil {
		return nil, fmt.Errorf("node group %s/%s not found", clusterName, nodeGroupName)
	}
	return nodeGroupFromAWS(output.Nodegroup), nil
}

// ListAllNodeGroups returns detailed information about all node groups for a cluster.
func (s *Service) ListAllNodeGroups(ctx context.Context, clusterName string) ([]NodeGroup, error) {
	names, err := s.ListNodeGroups(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	nodeGroups := make([]NodeGroup, 0, len(names))
	for _, name := range names {
		ng, err := s.GetNodeGroup(ctx, clusterName, name)
		if err != nil {
			continue
		}
		nodeGroups = append(nodeGroups, *ng)
	}
	return nodeGroups, nil
}

// ListFargateProfiles returns all Fargate profile names for a cluster.
func (s *Service) ListFargateProfiles(ctx context.Context, clusterName string) ([]string, error) {
	var profiles []string
	var nextToken *string

	for {
		input := &eks.ListFargateProfilesInput{
			ClusterName: aws.String(clusterName),
			NextToken:   nextToken,
		}
		output, err := s.client.ListFargateProfiles(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("listing Fargate profiles for %s: %w", clusterName, err)
		}

		profiles = append(profiles, output.FargateProfileNames...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return profiles, nil
}

// GetFargateProfile returns detailed information about a Fargate profile.
func (s *Service) GetFargateProfile(ctx context.Context, clusterName, profileName string) (*FargateProfile, error) {
	input := &eks.DescribeFargateProfileInput{
		ClusterName:        aws.String(clusterName),
		FargateProfileName: aws.String(profileName),
	}
	output, err := s.client.DescribeFargateProfile(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing Fargate profile %s/%s: %w", clusterName, profileName, err)
	}
	if output.FargateProfile == nil {
		return nil, fmt.Errorf("Fargate profile %s/%s not found", clusterName, profileName)
	}
	return fargateProfileFromAWS(output.FargateProfile), nil
}

// ListAllFargateProfiles returns detailed information about all Fargate profiles for a cluster.
func (s *Service) ListAllFargateProfiles(ctx context.Context, clusterName string) ([]FargateProfile, error) {
	names, err := s.ListFargateProfiles(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	profiles := make([]FargateProfile, 0, len(names))
	for _, name := range names {
		fp, err := s.GetFargateProfile(ctx, clusterName, name)
		if err != nil {
			continue
		}
		profiles = append(profiles, *fp)
	}
	return profiles, nil
}

// clusterFromAWS converts an AWS EKS cluster to our internal representation.
func clusterFromAWS(c *types.Cluster) *Cluster {
	cluster := &Cluster{
		Name:    aws.ToString(c.Name),
		ARN:     aws.ToString(c.Arn),
		Status:  string(c.Status),
		Version: aws.ToString(c.Version),
		RoleARN: aws.ToString(c.RoleArn),
		Tags:    c.Tags,
	}

	if c.PlatformVersion != nil {
		cluster.PlatformVersion = *c.PlatformVersion
	}
	if c.Endpoint != nil {
		cluster.Endpoint = *c.Endpoint
	}
	if c.CreatedAt != nil {
		cluster.CreatedAt = *c.CreatedAt
	}

	// Certificate Authority
	if c.CertificateAuthority != nil && c.CertificateAuthority.Data != nil {
		// Store first 64 chars + ... for display
		data := *c.CertificateAuthority.Data
		if len(data) > 64 {
			cluster.CertificateAuthorityData = data[:64] + "..."
		} else {
			cluster.CertificateAuthorityData = data
		}
	}

	// VPC/Networking config
	if c.ResourcesVpcConfig != nil {
		cluster.VPCID = aws.ToString(c.ResourcesVpcConfig.VpcId)
		cluster.SubnetIDs = c.ResourcesVpcConfig.SubnetIds
		cluster.SecurityGroupIDs = c.ResourcesVpcConfig.SecurityGroupIds
		cluster.ClusterSecurityGroup = aws.ToString(c.ResourcesVpcConfig.ClusterSecurityGroupId)
		cluster.EndpointPublicAccess = c.ResourcesVpcConfig.EndpointPublicAccess
		cluster.EndpointPrivateAccess = c.ResourcesVpcConfig.EndpointPrivateAccess
		cluster.PublicAccessCidrs = c.ResourcesVpcConfig.PublicAccessCidrs
	}

	// Kubernetes network config
	if c.KubernetesNetworkConfig != nil {
		cluster.ServiceIpv4Cidr = aws.ToString(c.KubernetesNetworkConfig.ServiceIpv4Cidr)
		cluster.ServiceIpv6Cidr = aws.ToString(c.KubernetesNetworkConfig.ServiceIpv6Cidr)
		cluster.IpFamily = string(c.KubernetesNetworkConfig.IpFamily)
	}

	// Logging
	if c.Logging != nil {
		for _, logSetup := range c.Logging.ClusterLogging {
			if logSetup.Enabled != nil && *logSetup.Enabled {
				for _, logType := range logSetup.Types {
					cluster.LoggingEnabled = append(cluster.LoggingEnabled, string(logType))
				}
			}
		}
	}

	// Encryption
	if len(c.EncryptionConfig) > 0 {
		if c.EncryptionConfig[0].Provider != nil {
			cluster.EncryptionKeyARN = aws.ToString(c.EncryptionConfig[0].Provider.KeyArn)
		}
		cluster.EncryptionResources = c.EncryptionConfig[0].Resources
	}

	// OIDC / Identity
	if c.Identity != nil && c.Identity.Oidc != nil {
		cluster.OIDCIssuer = aws.ToString(c.Identity.Oidc.Issuer)
	}

	// Access Config
	if c.AccessConfig != nil {
		cluster.AuthenticationMode = string(c.AccessConfig.AuthenticationMode)
		if c.AccessConfig.BootstrapClusterCreatorAdminPermissions != nil {
			cluster.BootstrapClusterCreatorAdminPermissions = *c.AccessConfig.BootstrapClusterCreatorAdminPermissions
		}
	}

	// Upgrade Policy
	if c.UpgradePolicy != nil {
		cluster.SupportType = string(c.UpgradePolicy.SupportType)
	}

	// Zonal Shift Config
	if c.ZonalShiftConfig != nil && c.ZonalShiftConfig.Enabled != nil {
		cluster.ZonalShiftEnabled = *c.ZonalShiftConfig.Enabled
	}

	// Compute Config (EKS Auto Mode)
	if c.ComputeConfig != nil {
		if c.ComputeConfig.Enabled != nil {
			cluster.ComputeConfigEnabled = *c.ComputeConfig.Enabled
		}
		cluster.NodePools = c.ComputeConfig.NodePools
		cluster.NodeRoleARN = aws.ToString(c.ComputeConfig.NodeRoleArn)
	}

	// Storage Config
	if c.StorageConfig != nil && c.StorageConfig.BlockStorage != nil {
		if c.StorageConfig.BlockStorage.Enabled != nil {
			cluster.BlockStorageEnabled = *c.StorageConfig.BlockStorage.Enabled
		}
	}

	// Remote Network Config
	if c.RemoteNetworkConfig != nil {
		for _, rn := range c.RemoteNetworkConfig.RemoteNodeNetworks {
			cluster.RemoteNodeNetworks = append(cluster.RemoteNodeNetworks, rn.Cidrs...)
		}
		for _, rp := range c.RemoteNetworkConfig.RemotePodNetworks {
			cluster.RemotePodNetworks = append(cluster.RemotePodNetworks, rp.Cidrs...)
		}
	}

	// Outpost Config
	if c.OutpostConfig != nil {
		cluster.OutpostARNs = c.OutpostConfig.OutpostArns
		cluster.ControlPlaneInstanceType = aws.ToString(c.OutpostConfig.ControlPlaneInstanceType)
		if c.OutpostConfig.ControlPlanePlacement != nil {
			cluster.ControlPlanePlacement = aws.ToString(c.OutpostConfig.ControlPlanePlacement.GroupName)
		}
	}

	return cluster
}

// nodeGroupFromAWS converts an AWS EKS node group to our internal representation.
func nodeGroupFromAWS(ng *types.Nodegroup) *NodeGroup {
	nodeGroup := &NodeGroup{
		Name:          aws.ToString(ng.NodegroupName),
		ClusterName:   aws.ToString(ng.ClusterName),
		ARN:           aws.ToString(ng.NodegroupArn),
		Status:        string(ng.Status),
		CapacityType:  string(ng.CapacityType),
		InstanceTypes: ng.InstanceTypes,
		AmiType:       string(ng.AmiType),
		Subnets:       ng.Subnets,
		Tags:          ng.Tags,
	}

	if ng.DiskSize != nil {
		nodeGroup.DiskSize = *ng.DiskSize
	}
	if ng.ScalingConfig != nil {
		if ng.ScalingConfig.DesiredSize != nil {
			nodeGroup.DesiredSize = *ng.ScalingConfig.DesiredSize
		}
		if ng.ScalingConfig.MinSize != nil {
			nodeGroup.MinSize = *ng.ScalingConfig.MinSize
		}
		if ng.ScalingConfig.MaxSize != nil {
			nodeGroup.MaxSize = *ng.ScalingConfig.MaxSize
		}
	}
	if ng.CreatedAt != nil {
		nodeGroup.CreatedAt = *ng.CreatedAt
	}

	return nodeGroup
}

// fargateProfileFromAWS converts an AWS EKS Fargate profile to our internal representation.
func fargateProfileFromAWS(fp *types.FargateProfile) *FargateProfile {
	profile := &FargateProfile{
		Name:                aws.ToString(fp.FargateProfileName),
		ClusterName:         aws.ToString(fp.ClusterName),
		ARN:                 aws.ToString(fp.FargateProfileArn),
		Status:              string(fp.Status),
		PodExecutionRoleARN: aws.ToString(fp.PodExecutionRoleArn),
		Subnets:             fp.Subnets,
		Tags:                fp.Tags,
	}

	if fp.CreatedAt != nil {
		profile.CreatedAt = *fp.CreatedAt
	}

	for _, sel := range fp.Selectors {
		profile.Selectors = append(profile.Selectors, FargateSelector{
			Namespace: aws.ToString(sel.Namespace),
			Labels:    sel.Labels,
		})
	}

	return profile
}
