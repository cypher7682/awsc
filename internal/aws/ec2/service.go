// Package ec2 provides the EC2 service layer for awsc.
package ec2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Instance represents a simplified EC2 instance for display.
type Instance struct {
	InstanceID       string
	Name             string
	State            string
	Type             string
	PrivateIP        string
	PublicIP         string
	VPCID            string
	SubnetID         string
	SecurityGroupIDs []string
	LaunchTime       time.Time
	Platform         string
	AZ               string
	AMI              string
	KeyName          string
	IAMRole          string
	Tags             map[string]string
}

// SecurityGroup represents a simplified security group.
type SecurityGroup struct {
	GroupID     string
	GroupName   string
	Description string
	VPCID       string
	IngressRules []SGRule
	EgressRules  []SGRule
	Tags         map[string]string
}

// SGRule represents a single security group rule.
type SGRule struct {
	Protocol   string
	FromPort   int32
	ToPort     int32
	Source     string // CIDR or security group ID
	Description string
}

// VPC represents a simplified VPC.
type VPC struct {
	VPCID     string
	CIDRBlock string
	Name      string
	State     string
	IsDefault bool
	Tags      map[string]string
}

// Subnet represents a simplified subnet.
type Subnet struct {
	SubnetID         string
	VPCID            string
	CIDRBlock        string
	AZ               string
	Name             string
	AvailableIPs     int32
	MapPublicIP      bool
	Tags             map[string]string
}

// InstanceTypeInfo represents an EC2 instance type with its specifications.
type InstanceTypeInfo struct {
	Name               string   // e.g., "t3.micro"
	FreeTierEligible   bool
	VCPUs              int32
	Architectures      []string // e.g., ["x86_64"], ["arm64"]
	MemoryMiB          int64
	StorageGB          int64    // 0 if EBS-only
	StorageType        string   // "ssd", "hdd", "nvme", or ""
	NetworkPerformance string   // e.g., "Up to 5 Gigabit"
	CurrentGeneration  bool
	BareMetal          bool
	Hypervisor         string   // "xen", "nitro", or ""
}

// LaunchTemplate represents an EC2 launch template for the list view.
type LaunchTemplate struct {
	LaunchTemplateID   string
	LaunchTemplateName string
	DefaultVersion     int64
	LatestVersion      int64
	CreateTime         time.Time
	CreatedBy          string
	Tags               map[string]string
}

// LaunchTemplateDetail contains full launch template info including version data.
type LaunchTemplateDetail struct {
	LaunchTemplate
	// Version data (from default version)
	VersionDescription string
	InstanceType       string
	ImageID            string
	KeyName            string
	SecurityGroupIDs   []string
	SecurityGroupNames []string
	// Network interfaces
	NetworkInterfaces []LTNetworkInterface
	// Storage
	BlockDeviceMappings []LTBlockDevice
	// Placement
	AvailabilityZone string
	Tenancy          string
	// IAM
	IAMInstanceProfile string
	// Monitoring
	MonitoringEnabled bool
	// Metadata
	UserData            string
	DisableAPIStop      bool
	DisableAPITerminate bool
	EBSOptimized        bool
	// Tags for instances launched
	TagSpecifications []LTTagSpec
}

// LTNetworkInterface represents a network interface in a launch template.
type LTNetworkInterface struct {
	DeviceIndex              int32
	SubnetID                 string
	AssociatePublicIPAddress bool
	SecurityGroupIDs         []string
	DeleteOnTermination      bool
	Description              string
}

// LTBlockDevice represents a block device mapping in a launch template.
type LTBlockDevice struct {
	DeviceName          string
	VolumeType          string
	VolumeSize          int32
	IOPS                int32
	Throughput          int32
	Encrypted           bool
	DeleteOnTermination bool
	SnapshotID          string
}

// LTTagSpec represents tag specifications for launched resources.
type LTTagSpec struct {
	ResourceType string
	Tags         map[string]string
}

// SpotInstanceRequest represents an EC2 spot instance request.
type SpotInstanceRequest struct {
	RequestID        string
	State            string // open, active, closed, cancelled, failed
	StatusCode       string
	StatusMessage    string
	InstanceID       string // empty if not yet fulfilled
	InstanceType     string
	SpotPrice        string
	CreateTime       time.Time
	ValidFrom        time.Time
	ValidUntil       time.Time
	LaunchGroup      string
	AvailabilityZone string
	ProductDescription string
	Type             string // one-time, persistent
	Tags             map[string]string
}

// SpotInstanceRequestDetail contains full spot request info.
type SpotInstanceRequestDetail struct {
	SpotInstanceRequest
	// Launch specification
	ImageID            string
	KeyName            string
	SecurityGroupIDs   []string
	SubnetID           string
	IAMInstanceProfile string
	UserData           string
	EBSOptimized       bool
	Monitoring         bool
	// Block devices
	BlockDeviceMappings []LTBlockDevice
	// Fault info
	FaultCode    string
	FaultMessage string
}

// EC2API defines the interface for EC2 operations (for testability).
type EC2API interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeLaunchTemplates(ctx context.Context, params *ec2.DescribeLaunchTemplatesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error)
	DescribeLaunchTemplateVersions(ctx context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
	DescribeSpotInstanceRequests(ctx context.Context, params *ec2.DescribeSpotInstanceRequestsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSpotInstanceRequestsOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error)
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DeleteTags(ctx context.Context, params *ec2.DeleteTagsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteTagsOutput, error)
}

// Service provides EC2 operations.
type Service struct {
	client EC2API
}

// NewService creates a new EC2 service from an AWS config.
func NewService(cfg aws.Config) *Service {
	return &Service{
		client: ec2.NewFromConfig(cfg),
	}
}

// NewServiceFromClient creates a new EC2 service from a pre-built SDK client.
func NewServiceFromClient(client *ec2.Client) *Service {
	return &Service{client: client}
}

// NewServiceWithClient creates a new EC2 service with a custom client (for testing).
func NewServiceWithClient(client EC2API) *Service {
	return &Service{client: client}
}

// ListInstances returns all EC2 instances, optionally filtered.
func (s *Service) ListInstances(ctx context.Context, filters []types.Filter) ([]Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	if len(filters) > 0 {
		input.Filters = filters
	}

	var instances []Instance
	paginator := ec2.NewDescribeInstancesPaginator(s.client.(*ec2.Client), input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing instances: %w", err)
		}
		for _, reservation := range output.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, instanceFromAWS(inst))
			}
		}
	}

	return instances, nil
}

// ListInstancesRaw returns all EC2 instances using the EC2API interface directly (no paginator).
func (s *Service) ListInstancesRaw(ctx context.Context, filters []types.Filter) ([]Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	if len(filters) > 0 {
		input.Filters = filters
	}

	var instances []Instance
	var nextToken *string

	for {
		input.NextToken = nextToken
		output, err := s.client.DescribeInstances(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing instances: %w", err)
		}
		for _, reservation := range output.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, instanceFromAWS(inst))
			}
		}
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return instances, nil
}

// GetInstance returns a single instance by ID.
func (s *Service) GetInstance(ctx context.Context, instanceID string) (*Instance, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	output, err := s.client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing instance %s: %w", instanceID, err)
	}
	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			instance := instanceFromAWS(inst)
			return &instance, nil
		}
	}
	return nil, fmt.Errorf("instance %s not found", instanceID)
}

// TerminateInstance terminates an EC2 instance.
func (s *Service) TerminateInstance(ctx context.Context, instanceID string) error {
	_, err := s.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("terminating instance %s: %w", instanceID, err)
	}
	return nil
}

// RebootInstance reboots an EC2 instance.
func (s *Service) RebootInstance(ctx context.Context, instanceID string) error {
	_, err := s.client.RebootInstances(ctx, &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("rebooting instance %s: %w", instanceID, err)
	}
	return nil
}

// StopInstance stops an EC2 instance.
func (s *Service) StopInstance(ctx context.Context, instanceID string) error {
	_, err := s.client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("stopping instance %s: %w", instanceID, err)
	}
	return nil
}

// ForceStopInstance forcibly stops an EC2 instance (may cause data loss).
func (s *Service) ForceStopInstance(ctx context.Context, instanceID string) error {
	force := true
	_, err := s.client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
		Force:       &force,
	})
	if err != nil {
		return fmt.Errorf("force stopping instance %s: %w", instanceID, err)
	}
	return nil
}

// StartInstance starts an EC2 instance.
func (s *Service) StartInstance(ctx context.Context, instanceID string) error {
	_, err := s.client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("starting instance %s: %w", instanceID, err)
	}
	return nil
}

// ListSecurityGroups returns security groups, optionally filtered.
func (s *Service) ListSecurityGroups(ctx context.Context, groupIDs []string) ([]SecurityGroup, error) {
	input := &ec2.DescribeSecurityGroupsInput{}
	if len(groupIDs) > 0 {
		input.GroupIds = groupIDs
	}

	output, err := s.client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing security groups: %w", err)
	}

	var groups []SecurityGroup
	for _, sg := range output.SecurityGroups {
		groups = append(groups, securityGroupFromAWS(sg))
	}
	return groups, nil
}

// ListVPCs returns all VPCs.
func (s *Service) ListVPCs(ctx context.Context) ([]VPC, error) {
	output, err := s.client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("describing VPCs: %w", err)
	}

	var vpcs []VPC
	for _, v := range output.Vpcs {
		vpcs = append(vpcs, vpcFromAWS(v))
	}
	return vpcs, nil
}

// ListSubnets returns subnets, optionally filtered by VPC ID.
func (s *Service) ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error) {
	input := &ec2.DescribeSubnetsInput{}
	if vpcID != "" {
		input.Filters = []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		}
	}

	output, err := s.client.DescribeSubnets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing subnets: %w", err)
	}

	var subnets []Subnet
	for _, sn := range output.Subnets {
		subnets = append(subnets, subnetFromAWS(sn))
	}
	return subnets, nil
}

// ListInstanceTypes returns all available EC2 instance types.
func (s *Service) ListInstanceTypes(ctx context.Context) ([]InstanceTypeInfo, error) {
	var instanceTypes []InstanceTypeInfo
	var nextToken *string

	for {
		input := &ec2.DescribeInstanceTypesInput{
			NextToken: nextToken,
		}

		output, err := s.client.DescribeInstanceTypes(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing instance types: %w", err)
		}

		for _, it := range output.InstanceTypes {
			instanceTypes = append(instanceTypes, instanceTypeFromAWS(it))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return instanceTypes, nil
}

// ListLaunchTemplates returns all launch templates.
func (s *Service) ListLaunchTemplates(ctx context.Context) ([]LaunchTemplate, error) {
	var templates []LaunchTemplate
	var nextToken *string

	for {
		input := &ec2.DescribeLaunchTemplatesInput{
			NextToken: nextToken,
		}

		output, err := s.client.DescribeLaunchTemplates(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing launch templates: %w", err)
		}

		for _, lt := range output.LaunchTemplates {
			templates = append(templates, launchTemplateFromAWS(lt))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return templates, nil
}

// GetLaunchTemplate returns a launch template with its default version details.
func (s *Service) GetLaunchTemplate(ctx context.Context, templateID string) (*LaunchTemplateDetail, error) {
	// First get the template itself
	ltOutput, err := s.client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateIds: []string{templateID},
	})
	if err != nil {
		return nil, fmt.Errorf("describing launch template %s: %w", templateID, err)
	}
	if len(ltOutput.LaunchTemplates) == 0 {
		return nil, fmt.Errorf("launch template %s not found", templateID)
	}

	lt := launchTemplateFromAWS(ltOutput.LaunchTemplates[0])

	// Get the default version details
	versOutput, err := s.client.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(templateID),
		Versions:         []string{"$Default"},
	})
	if err != nil {
		return nil, fmt.Errorf("describing launch template version: %w", err)
	}

	detail := &LaunchTemplateDetail{
		LaunchTemplate: lt,
	}

	if len(versOutput.LaunchTemplateVersions) > 0 {
		ver := versOutput.LaunchTemplateVersions[0]
		detail.VersionDescription = aws.ToString(ver.VersionDescription)

		if data := ver.LaunchTemplateData; data != nil {
			detail.InstanceType = string(data.InstanceType)
			detail.ImageID = aws.ToString(data.ImageId)
			detail.KeyName = aws.ToString(data.KeyName)
			detail.EBSOptimized = aws.ToBool(data.EbsOptimized)
			detail.DisableAPITerminate = aws.ToBool(data.DisableApiTermination)
			detail.DisableAPIStop = aws.ToBool(data.DisableApiStop)

			// Security groups
			for _, sg := range data.SecurityGroupIds {
				detail.SecurityGroupIDs = append(detail.SecurityGroupIDs, sg)
			}
			for _, sg := range data.SecurityGroups {
				detail.SecurityGroupNames = append(detail.SecurityGroupNames, sg)
			}

			// Placement
			if data.Placement != nil {
				detail.AvailabilityZone = aws.ToString(data.Placement.AvailabilityZone)
				detail.Tenancy = string(data.Placement.Tenancy)
			}

			// IAM
			if data.IamInstanceProfile != nil {
				if data.IamInstanceProfile.Arn != nil {
					detail.IAMInstanceProfile = aws.ToString(data.IamInstanceProfile.Arn)
				} else if data.IamInstanceProfile.Name != nil {
					detail.IAMInstanceProfile = aws.ToString(data.IamInstanceProfile.Name)
				}
			}

			// Monitoring
			if data.Monitoring != nil {
				detail.MonitoringEnabled = aws.ToBool(data.Monitoring.Enabled)
			}

			// User data (base64)
			detail.UserData = aws.ToString(data.UserData)

			// Network interfaces
			for _, ni := range data.NetworkInterfaces {
				ltNI := LTNetworkInterface{
					DeviceIndex:              aws.ToInt32(ni.DeviceIndex),
					SubnetID:                 aws.ToString(ni.SubnetId),
					AssociatePublicIPAddress: aws.ToBool(ni.AssociatePublicIpAddress),
					DeleteOnTermination:      aws.ToBool(ni.DeleteOnTermination),
					Description:              aws.ToString(ni.Description),
				}
				for _, sg := range ni.Groups {
					ltNI.SecurityGroupIDs = append(ltNI.SecurityGroupIDs, sg)
				}
				detail.NetworkInterfaces = append(detail.NetworkInterfaces, ltNI)
			}

			// Block devices
			for _, bd := range data.BlockDeviceMappings {
				ltBD := LTBlockDevice{
					DeviceName: aws.ToString(bd.DeviceName),
				}
				if bd.Ebs != nil {
					ltBD.VolumeType = string(bd.Ebs.VolumeType)
					ltBD.VolumeSize = aws.ToInt32(bd.Ebs.VolumeSize)
					ltBD.IOPS = aws.ToInt32(bd.Ebs.Iops)
					ltBD.Throughput = aws.ToInt32(bd.Ebs.Throughput)
					ltBD.Encrypted = aws.ToBool(bd.Ebs.Encrypted)
					ltBD.DeleteOnTermination = aws.ToBool(bd.Ebs.DeleteOnTermination)
					ltBD.SnapshotID = aws.ToString(bd.Ebs.SnapshotId)
				}
				detail.BlockDeviceMappings = append(detail.BlockDeviceMappings, ltBD)
			}

			// Tag specifications
			for _, ts := range data.TagSpecifications {
				ltTS := LTTagSpec{
					ResourceType: string(ts.ResourceType),
					Tags:         make(map[string]string),
				}
				for _, tag := range ts.Tags {
					ltTS.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}
				detail.TagSpecifications = append(detail.TagSpecifications, ltTS)
			}
		}
	}

	return detail, nil
}

// ListSpotInstanceRequests returns all spot instance requests.
func (s *Service) ListSpotInstanceRequests(ctx context.Context) ([]SpotInstanceRequest, error) {
	var requests []SpotInstanceRequest
	var nextToken *string

	for {
		input := &ec2.DescribeSpotInstanceRequestsInput{
			NextToken: nextToken,
		}

		output, err := s.client.DescribeSpotInstanceRequests(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing spot instance requests: %w", err)
		}

		for _, sir := range output.SpotInstanceRequests {
			requests = append(requests, spotInstanceRequestFromAWS(sir))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return requests, nil
}

// GetSpotInstanceRequest returns a spot instance request with full details.
func (s *Service) GetSpotInstanceRequest(ctx context.Context, requestID string) (*SpotInstanceRequestDetail, error) {
	output, err := s.client.DescribeSpotInstanceRequests(ctx, &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []string{requestID},
	})
	if err != nil {
		return nil, fmt.Errorf("describing spot instance request %s: %w", requestID, err)
	}
	if len(output.SpotInstanceRequests) == 0 {
		return nil, fmt.Errorf("spot instance request %s not found", requestID)
	}

	sir := output.SpotInstanceRequests[0]
	basic := spotInstanceRequestFromAWS(sir)

	detail := &SpotInstanceRequestDetail{
		SpotInstanceRequest: basic,
	}

	// Launch specification details
	if spec := sir.LaunchSpecification; spec != nil {
		detail.ImageID = aws.ToString(spec.ImageId)
		detail.KeyName = aws.ToString(spec.KeyName)
		detail.SubnetID = aws.ToString(spec.SubnetId)
		detail.EBSOptimized = aws.ToBool(spec.EbsOptimized)

		for _, sg := range spec.SecurityGroups {
			detail.SecurityGroupIDs = append(detail.SecurityGroupIDs, aws.ToString(sg.GroupId))
		}

		if spec.IamInstanceProfile != nil {
			if spec.IamInstanceProfile.Arn != nil {
				detail.IAMInstanceProfile = aws.ToString(spec.IamInstanceProfile.Arn)
			} else if spec.IamInstanceProfile.Name != nil {
				detail.IAMInstanceProfile = aws.ToString(spec.IamInstanceProfile.Name)
			}
		}

		if spec.Monitoring != nil {
			detail.Monitoring = aws.ToBool(spec.Monitoring.Enabled)
		}

		detail.UserData = aws.ToString(spec.UserData)

		// Block devices
		for _, bd := range spec.BlockDeviceMappings {
			ltBD := LTBlockDevice{
				DeviceName: aws.ToString(bd.DeviceName),
			}
			if bd.Ebs != nil {
				ltBD.VolumeType = string(bd.Ebs.VolumeType)
				ltBD.VolumeSize = aws.ToInt32(bd.Ebs.VolumeSize)
				ltBD.IOPS = aws.ToInt32(bd.Ebs.Iops)
				ltBD.Encrypted = aws.ToBool(bd.Ebs.Encrypted)
				ltBD.DeleteOnTermination = aws.ToBool(bd.Ebs.DeleteOnTermination)
				ltBD.SnapshotID = aws.ToString(bd.Ebs.SnapshotId)
			}
			detail.BlockDeviceMappings = append(detail.BlockDeviceMappings, ltBD)
		}
	}

	// Fault info
	if sir.Fault != nil {
		detail.FaultCode = aws.ToString(sir.Fault.Code)
		detail.FaultMessage = aws.ToString(sir.Fault.Message)
	}

	return detail, nil
}

// AddIngressRule adds an ingress rule to a security group.
func (s *Service) AddIngressRule(ctx context.Context, groupID string, rule SGRule) error {
	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(groupID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String(rule.Protocol),
				FromPort:   aws.Int32(rule.FromPort),
				ToPort:     aws.Int32(rule.ToPort),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String(rule.Source),
						Description: aws.String(rule.Description),
					},
				},
			},
		},
	}

	_, err := s.client.AuthorizeSecurityGroupIngress(ctx, input)
	if err != nil {
		return fmt.Errorf("adding ingress rule to %s: %w", groupID, err)
	}
	return nil
}

// RemoveIngressRule removes an ingress rule from a security group.
func (s *Service) RemoveIngressRule(ctx context.Context, groupID string, rule SGRule) error {
	input := &ec2.RevokeSecurityGroupIngressInput{
		GroupId: aws.String(groupID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String(rule.Protocol),
				FromPort:   aws.Int32(rule.FromPort),
				ToPort:     aws.Int32(rule.ToPort),
				IpRanges: []types.IpRange{
					{
						CidrIp: aws.String(rule.Source),
					},
				},
			},
		},
	}

	_, err := s.client.RevokeSecurityGroupIngress(ctx, input)
	if err != nil {
		return fmt.Errorf("removing ingress rule from %s: %w", groupID, err)
	}
	return nil
}

// instanceFromAWS converts an AWS EC2 instance to our internal representation.
func instanceFromAWS(inst types.Instance) Instance {
	var az string
	if inst.Placement != nil {
		az = aws.ToString(inst.Placement.AvailabilityZone)
	}

	i := Instance{
		InstanceID: aws.ToString(inst.InstanceId),
		Type:       string(inst.InstanceType),
		PrivateIP:  aws.ToString(inst.PrivateIpAddress),
		PublicIP:   aws.ToString(inst.PublicIpAddress),
		VPCID:      aws.ToString(inst.VpcId),
		SubnetID:   aws.ToString(inst.SubnetId),
		Platform:   aws.ToString(inst.PlatformDetails),
		AZ:         az,
		AMI:        aws.ToString(inst.ImageId),
		KeyName:    aws.ToString(inst.KeyName),
		Tags:       make(map[string]string),
	}

	if inst.State != nil {
		i.State = string(inst.State.Name)
	}
	if inst.LaunchTime != nil {
		i.LaunchTime = *inst.LaunchTime
	}

	for _, sg := range inst.SecurityGroups {
		i.SecurityGroupIDs = append(i.SecurityGroupIDs, aws.ToString(sg.GroupId))
	}

	for _, tag := range inst.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		i.Tags[key] = value
		if key == "Name" {
			i.Name = value
		}
	}

	if inst.IamInstanceProfile != nil {
		i.IAMRole = aws.ToString(inst.IamInstanceProfile.Arn)
	}

	return i
}

// securityGroupFromAWS converts an AWS security group to our internal representation.
func securityGroupFromAWS(sg types.SecurityGroup) SecurityGroup {
	g := SecurityGroup{
		GroupID:     aws.ToString(sg.GroupId),
		GroupName:   aws.ToString(sg.GroupName),
		Description: aws.ToString(sg.Description),
		VPCID:       aws.ToString(sg.VpcId),
		Tags:        make(map[string]string),
	}

	for _, perm := range sg.IpPermissions {
		for _, ipRange := range perm.IpRanges {
			g.IngressRules = append(g.IngressRules, SGRule{
				Protocol:    aws.ToString(perm.IpProtocol),
				FromPort:    aws.ToInt32(perm.FromPort),
				ToPort:      aws.ToInt32(perm.ToPort),
				Source:      aws.ToString(ipRange.CidrIp),
				Description: aws.ToString(ipRange.Description),
			})
		}
		for _, group := range perm.UserIdGroupPairs {
			g.IngressRules = append(g.IngressRules, SGRule{
				Protocol:    aws.ToString(perm.IpProtocol),
				FromPort:    aws.ToInt32(perm.FromPort),
				ToPort:      aws.ToInt32(perm.ToPort),
				Source:      aws.ToString(group.GroupId),
				Description: aws.ToString(group.Description),
			})
		}
	}

	for _, perm := range sg.IpPermissionsEgress {
		for _, ipRange := range perm.IpRanges {
			g.EgressRules = append(g.EgressRules, SGRule{
				Protocol:    aws.ToString(perm.IpProtocol),
				FromPort:    aws.ToInt32(perm.FromPort),
				ToPort:      aws.ToInt32(perm.ToPort),
				Source:      aws.ToString(ipRange.CidrIp),
				Description: aws.ToString(ipRange.Description),
			})
		}
	}

	for _, tag := range sg.Tags {
		g.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return g
}

// vpcFromAWS converts an AWS VPC to our internal representation.
func vpcFromAWS(v types.Vpc) VPC {
	vpc := VPC{
		VPCID:     aws.ToString(v.VpcId),
		CIDRBlock: aws.ToString(v.CidrBlock),
		State:     string(v.State),
		IsDefault: aws.ToBool(v.IsDefault),
		Tags:      make(map[string]string),
	}

	for _, tag := range v.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		vpc.Tags[key] = value
		if key == "Name" {
			vpc.Name = value
		}
	}

	return vpc
}

// subnetFromAWS converts an AWS subnet to our internal representation.
func subnetFromAWS(sn types.Subnet) Subnet {
	subnet := Subnet{
		SubnetID:     aws.ToString(sn.SubnetId),
		VPCID:        aws.ToString(sn.VpcId),
		CIDRBlock:    aws.ToString(sn.CidrBlock),
		AZ:           aws.ToString(sn.AvailabilityZone),
		AvailableIPs: aws.ToInt32(sn.AvailableIpAddressCount),
		MapPublicIP:  aws.ToBool(sn.MapPublicIpOnLaunch),
		Tags:         make(map[string]string),
	}

	for _, tag := range sn.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		subnet.Tags[key] = value
		if key == "Name" {
			subnet.Name = value
		}
	}

	return subnet
}

// CreateTag adds or updates a tag on a resource.
func (s *Service) CreateTag(ctx context.Context, resourceID, key, value string) error {
	_, err := s.client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{resourceID},
		Tags: []types.Tag{
			{Key: aws.String(key), Value: aws.String(value)},
		},
	})
	return err
}

// DeleteTag removes a tag from a resource.
func (s *Service) DeleteTag(ctx context.Context, resourceID, key string) error {
	_, err := s.client.DeleteTags(ctx, &ec2.DeleteTagsInput{
		Resources: []string{resourceID},
		Tags: []types.Tag{
			{Key: aws.String(key)},
		},
	})
	return err
}

// instanceTypeFromAWS converts an AWS InstanceTypeInfo to our internal representation.
func instanceTypeFromAWS(it types.InstanceTypeInfo) InstanceTypeInfo {
	info := InstanceTypeInfo{
		Name:              string(it.InstanceType),
		FreeTierEligible:  aws.ToBool(it.FreeTierEligible),
		CurrentGeneration: aws.ToBool(it.CurrentGeneration),
		BareMetal:         aws.ToBool(it.BareMetal),
	}

	// vCPUs
	if it.VCpuInfo != nil {
		info.VCPUs = aws.ToInt32(it.VCpuInfo.DefaultVCpus)
	}

	// Architectures
	if it.ProcessorInfo != nil {
		for _, arch := range it.ProcessorInfo.SupportedArchitectures {
			info.Architectures = append(info.Architectures, string(arch))
		}
	}

	// Memory
	if it.MemoryInfo != nil {
		info.MemoryMiB = aws.ToInt64(it.MemoryInfo.SizeInMiB)
	}

	// Storage
	if it.InstanceStorageInfo != nil {
		info.StorageGB = aws.ToInt64(it.InstanceStorageInfo.TotalSizeInGB)
		if it.InstanceStorageInfo.Disks != nil && len(it.InstanceStorageInfo.Disks) > 0 {
			info.StorageType = string(it.InstanceStorageInfo.Disks[0].Type)
		}
	}

	// Network
	if it.NetworkInfo != nil {
		info.NetworkPerformance = aws.ToString(it.NetworkInfo.NetworkPerformance)
	}

	// Hypervisor
	info.Hypervisor = string(it.Hypervisor)

	return info
}

// launchTemplateFromAWS converts an AWS LaunchTemplate to our internal representation.
func launchTemplateFromAWS(lt types.LaunchTemplate) LaunchTemplate {
	template := LaunchTemplate{
		LaunchTemplateID:   aws.ToString(lt.LaunchTemplateId),
		LaunchTemplateName: aws.ToString(lt.LaunchTemplateName),
		DefaultVersion:     aws.ToInt64(lt.DefaultVersionNumber),
		LatestVersion:      aws.ToInt64(lt.LatestVersionNumber),
		CreatedBy:          aws.ToString(lt.CreatedBy),
		Tags:               make(map[string]string),
	}

	if lt.CreateTime != nil {
		template.CreateTime = *lt.CreateTime
	}

	for _, tag := range lt.Tags {
		template.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return template
}

// spotInstanceRequestFromAWS converts an AWS SpotInstanceRequest to our internal representation.
func spotInstanceRequestFromAWS(sir types.SpotInstanceRequest) SpotInstanceRequest {
	req := SpotInstanceRequest{
		RequestID:          aws.ToString(sir.SpotInstanceRequestId),
		State:              string(sir.State),
		InstanceID:         aws.ToString(sir.InstanceId),
		SpotPrice:          aws.ToString(sir.SpotPrice),
		LaunchGroup:        aws.ToString(sir.LaunchGroup),
		ProductDescription: string(sir.ProductDescription),
		Type:               string(sir.Type),
		Tags:               make(map[string]string),
	}

	if sir.Status != nil {
		req.StatusCode = aws.ToString(sir.Status.Code)
		req.StatusMessage = aws.ToString(sir.Status.Message)
	}

	if sir.LaunchSpecification != nil {
		req.InstanceType = string(sir.LaunchSpecification.InstanceType)
		if sir.LaunchSpecification.Placement != nil {
			req.AvailabilityZone = aws.ToString(sir.LaunchSpecification.Placement.AvailabilityZone)
		}
	}

	if sir.CreateTime != nil {
		req.CreateTime = *sir.CreateTime
	}
	if sir.ValidFrom != nil {
		req.ValidFrom = *sir.ValidFrom
	}
	if sir.ValidUntil != nil {
		req.ValidUntil = *sir.ValidUntil
	}

	for _, tag := range sir.Tags {
		req.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return req
}
