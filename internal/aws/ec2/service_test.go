package ec2

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2Client implements EC2API for testing.
type mockEC2Client struct {
	describeInstancesOutput       *ec2.DescribeInstancesOutput
	describeInstancesError        error
	terminateInstancesOutput      *ec2.TerminateInstancesOutput
	terminateInstancesError       error
	rebootInstancesOutput         *ec2.RebootInstancesOutput
	rebootInstancesError          error
	stopInstancesOutput           *ec2.StopInstancesOutput
	stopInstancesError            error
	startInstancesOutput          *ec2.StartInstancesOutput
	startInstancesError           error
	describeSecurityGroupsOutput  *ec2.DescribeSecurityGroupsOutput
	describeSecurityGroupsError   error
	describeVpcsOutput            *ec2.DescribeVpcsOutput
	describeVpcsError             error
	describeSubnetsOutput         *ec2.DescribeSubnetsOutput
	describeSubnetsError          error
	authorizeIngressOutput        *ec2.AuthorizeSecurityGroupIngressOutput
	authorizeIngressError         error
	revokeIngressOutput           *ec2.RevokeSecurityGroupIngressOutput
	revokeIngressError            error
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.describeInstancesOutput, m.describeInstancesError
}

func (m *mockEC2Client) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	return m.terminateInstancesOutput, m.terminateInstancesError
}

func (m *mockEC2Client) RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error) {
	return m.rebootInstancesOutput, m.rebootInstancesError
}

func (m *mockEC2Client) StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	return m.stopInstancesOutput, m.stopInstancesError
}

func (m *mockEC2Client) StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
	return m.startInstancesOutput, m.startInstancesError
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return m.describeSecurityGroupsOutput, m.describeSecurityGroupsError
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return m.describeVpcsOutput, m.describeVpcsError
}

func (m *mockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return m.describeSubnetsOutput, m.describeSubnetsError
}

func (m *mockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return m.authorizeIngressOutput, m.authorizeIngressError
}

func (m *mockEC2Client) RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	return m.revokeIngressOutput, m.revokeIngressError
}

func (m *mockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return &ec2.CreateTagsOutput{}, nil
}

func (m *mockEC2Client) DeleteTags(ctx context.Context, params *ec2.DeleteTagsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteTagsOutput, error) {
	return &ec2.DeleteTagsOutput{}, nil
}

func TestListInstancesRaw(t *testing.T) {
	now := time.Now()
	mock := &mockEC2Client{
		describeInstancesOutput: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId:       aws.String("i-1234567890abcdef0"),
							InstanceType:     types.InstanceTypeT3Micro,
							PrivateIpAddress: aws.String("10.0.1.100"),
							PublicIpAddress:   aws.String("54.123.45.67"),
							VpcId:            aws.String("vpc-abc123"),
							SubnetId:         aws.String("subnet-def456"),
							ImageId:          aws.String("ami-12345678"),
							KeyName:          aws.String("my-key"),
							LaunchTime:       &now,
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							Placement: &types.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
							SecurityGroups: []types.GroupIdentifier{
								{GroupId: aws.String("sg-111")},
								{GroupId: aws.String("sg-222")},
							},
							Tags: []types.Tag{
								{Key: aws.String("Name"), Value: aws.String("web-server-1")},
								{Key: aws.String("env"), Value: aws.String("production")},
							},
						},
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	instances, err := svc.ListInstancesRaw(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	inst := instances[0]
	if inst.InstanceID != "i-1234567890abcdef0" {
		t.Errorf("expected instance ID 'i-1234567890abcdef0', got '%s'", inst.InstanceID)
	}
	if inst.Name != "web-server-1" {
		t.Errorf("expected name 'web-server-1', got '%s'", inst.Name)
	}
	if inst.State != "running" {
		t.Errorf("expected state 'running', got '%s'", inst.State)
	}
	if inst.Type != "t3.micro" {
		t.Errorf("expected type 't3.micro', got '%s'", inst.Type)
	}
	if inst.PrivateIP != "10.0.1.100" {
		t.Errorf("expected private IP '10.0.1.100', got '%s'", inst.PrivateIP)
	}
	if inst.PublicIP != "54.123.45.67" {
		t.Errorf("expected public IP '54.123.45.67', got '%s'", inst.PublicIP)
	}
	if inst.VPCID != "vpc-abc123" {
		t.Errorf("expected VPC ID 'vpc-abc123', got '%s'", inst.VPCID)
	}
	if len(inst.SecurityGroupIDs) != 2 {
		t.Errorf("expected 2 security groups, got %d", len(inst.SecurityGroupIDs))
	}
	if inst.Tags["env"] != "production" {
		t.Errorf("expected tag 'env'='production', got '%s'", inst.Tags["env"])
	}
}

func TestGetInstance(t *testing.T) {
	mock := &mockEC2Client{
		describeInstancesOutput: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId:   aws.String("i-abc123"),
							InstanceType: types.InstanceTypeM5Large,
							State: &types.InstanceState{
								Name: types.InstanceStateNameStopped,
							},
							Placement: &types.Placement{
								AvailabilityZone: aws.String("eu-west-1a"),
							},
							Tags: []types.Tag{
								{Key: aws.String("Name"), Value: aws.String("db-server")},
							},
						},
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	inst, err := svc.GetInstance(context.Background(), "i-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstanceID != "i-abc123" {
		t.Errorf("expected 'i-abc123', got '%s'", inst.InstanceID)
	}
	if inst.Name != "db-server" {
		t.Errorf("expected 'db-server', got '%s'", inst.Name)
	}
	if inst.State != "stopped" {
		t.Errorf("expected 'stopped', got '%s'", inst.State)
	}
}

func TestGetInstance_NotFound(t *testing.T) {
	mock := &mockEC2Client{
		describeInstancesOutput: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{},
		},
	}

	svc := NewServiceWithClient(mock)
	_, err := svc.GetInstance(context.Background(), "i-nonexistent")
	if err == nil {
		t.Error("expected error for non-existent instance")
	}
}

func TestTerminateInstance(t *testing.T) {
	mock := &mockEC2Client{
		terminateInstancesOutput: &ec2.TerminateInstancesOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.TerminateInstance(context.Background(), "i-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRebootInstance(t *testing.T) {
	mock := &mockEC2Client{
		rebootInstancesOutput: &ec2.RebootInstancesOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.RebootInstance(context.Background(), "i-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopInstance(t *testing.T) {
	mock := &mockEC2Client{
		stopInstancesOutput: &ec2.StopInstancesOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.StopInstance(context.Background(), "i-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartInstance(t *testing.T) {
	mock := &mockEC2Client{
		startInstancesOutput: &ec2.StartInstancesOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.StartInstance(context.Background(), "i-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSecurityGroups(t *testing.T) {
	mock := &mockEC2Client{
		describeSecurityGroupsOutput: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []types.SecurityGroup{
				{
					GroupId:     aws.String("sg-123"),
					GroupName:   aws.String("web-sg"),
					Description: aws.String("Web security group"),
					VpcId:       aws.String("vpc-abc"),
					IpPermissions: []types.IpPermission{
						{
							IpProtocol: aws.String("tcp"),
							FromPort:   aws.Int32(443),
							ToPort:     aws.Int32(443),
							IpRanges: []types.IpRange{
								{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTPS")},
							},
						},
					},
					IpPermissionsEgress: []types.IpPermission{
						{
							IpProtocol: aws.String("-1"),
							FromPort:   aws.Int32(0),
							ToPort:     aws.Int32(0),
							IpRanges: []types.IpRange{
								{CidrIp: aws.String("0.0.0.0/0")},
							},
						},
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	groups, err := svc.ListSecurityGroups(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	sg := groups[0]
	if sg.GroupID != "sg-123" {
		t.Errorf("expected 'sg-123', got '%s'", sg.GroupID)
	}
	if sg.GroupName != "web-sg" {
		t.Errorf("expected 'web-sg', got '%s'", sg.GroupName)
	}
	if len(sg.IngressRules) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(sg.IngressRules))
	}
	if sg.IngressRules[0].FromPort != 443 {
		t.Errorf("expected from port 443, got %d", sg.IngressRules[0].FromPort)
	}
	if len(sg.EgressRules) != 1 {
		t.Fatalf("expected 1 egress rule, got %d", len(sg.EgressRules))
	}
}

func TestListVPCs(t *testing.T) {
	mock := &mockEC2Client{
		describeVpcsOutput: &ec2.DescribeVpcsOutput{
			Vpcs: []types.Vpc{
				{
					VpcId:     aws.String("vpc-main"),
					CidrBlock: aws.String("10.0.0.0/16"),
					State:     types.VpcStateAvailable,
					IsDefault: aws.Bool(true),
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-vpc")},
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	vpcs, err := svc.ListVPCs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vpcs) != 1 {
		t.Fatalf("expected 1 VPC, got %d", len(vpcs))
	}
	if vpcs[0].VPCID != "vpc-main" {
		t.Errorf("expected 'vpc-main', got '%s'", vpcs[0].VPCID)
	}
	if vpcs[0].Name != "main-vpc" {
		t.Errorf("expected name 'main-vpc', got '%s'", vpcs[0].Name)
	}
	if !vpcs[0].IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestListSubnets(t *testing.T) {
	mock := &mockEC2Client{
		describeSubnetsOutput: &ec2.DescribeSubnetsOutput{
			Subnets: []types.Subnet{
				{
					SubnetId:                aws.String("subnet-pub1"),
					VpcId:                   aws.String("vpc-main"),
					CidrBlock:              aws.String("10.0.1.0/24"),
					AvailabilityZone:       aws.String("us-east-1a"),
					AvailableIpAddressCount: aws.Int32(251),
					MapPublicIpOnLaunch:    aws.Bool(true),
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("public-1a")},
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	subnets, err := svc.ListSubnets(context.Background(), "vpc-main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(subnets) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(subnets))
	}
	if subnets[0].SubnetID != "subnet-pub1" {
		t.Errorf("expected 'subnet-pub1', got '%s'", subnets[0].SubnetID)
	}
	if subnets[0].Name != "public-1a" {
		t.Errorf("expected name 'public-1a', got '%s'", subnets[0].Name)
	}
	if !subnets[0].MapPublicIP {
		t.Error("expected MapPublicIP to be true")
	}
}

func TestAddIngressRule(t *testing.T) {
	mock := &mockEC2Client{
		authorizeIngressOutput: &ec2.AuthorizeSecurityGroupIngressOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.AddIngressRule(context.Background(), "sg-123", SGRule{
		Protocol:    "tcp",
		FromPort:    80,
		ToPort:      80,
		Source:      "10.0.0.0/8",
		Description: "HTTP from internal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveIngressRule(t *testing.T) {
	mock := &mockEC2Client{
		revokeIngressOutput: &ec2.RevokeSecurityGroupIngressOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.RemoveIngressRule(context.Background(), "sg-123", SGRule{
		Protocol: "tcp",
		FromPort: 80,
		ToPort:   80,
		Source:   "10.0.0.0/8",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
