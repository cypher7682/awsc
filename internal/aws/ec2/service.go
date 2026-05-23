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

// EC2API defines the interface for EC2 operations (for testability).
type EC2API interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error)
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
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
