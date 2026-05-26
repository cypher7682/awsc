package eks

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// mockEKSAPI implements EKSAPI for testing.
type mockEKSAPI struct {
	clusters        []string
	clusterDetails  map[string]*types.Cluster
	nodeGroups      map[string][]string
	nodeGroupDetails map[string]*types.Nodegroup
	fargateProfiles map[string][]string
	fargateDetails  map[string]*types.FargateProfile
}

func newMockEKSAPI() *mockEKSAPI {
	return &mockEKSAPI{
		clusters:        []string{"test-cluster"},
		clusterDetails:  make(map[string]*types.Cluster),
		nodeGroups:      make(map[string][]string),
		nodeGroupDetails: make(map[string]*types.Nodegroup),
		fargateProfiles: make(map[string][]string),
		fargateDetails:  make(map[string]*types.FargateProfile),
	}
}

func (m *mockEKSAPI) ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{
		Clusters: m.clusters,
	}, nil
}

func (m *mockEKSAPI) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	name := aws.ToString(params.Name)
	if c, ok := m.clusterDetails[name]; ok {
		return &eks.DescribeClusterOutput{Cluster: c}, nil
	}
	// Return a default cluster
	now := time.Now()
	return &eks.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Name:            params.Name,
			Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
			Status:          types.ClusterStatusActive,
			Version:         aws.String("1.28"),
			PlatformVersion: aws.String("eks.5"),
			Endpoint:        aws.String("https://ABC123.gr7.us-east-1.eks.amazonaws.com"),
			RoleArn:         aws.String("arn:aws:iam::123456789012:role/eksClusterRole"),
			CreatedAt:       &now,
			ResourcesVpcConfig: &types.VpcConfigResponse{
				VpcId:                  aws.String("vpc-12345678"),
				SubnetIds:              []string{"subnet-1", "subnet-2"},
				SecurityGroupIds:       []string{"sg-12345678"},
				EndpointPublicAccess:   true,
				EndpointPrivateAccess:  true,
				PublicAccessCidrs:      []string{"0.0.0.0/0"},
			},
			KubernetesNetworkConfig: &types.KubernetesNetworkConfigResponse{
				ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
				IpFamily:        types.IpFamilyIpv4,
			},
		},
	}, nil
}

func (m *mockEKSAPI) ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	clusterName := aws.ToString(params.ClusterName)
	return &eks.ListNodegroupsOutput{
		Nodegroups: m.nodeGroups[clusterName],
	}, nil
}

func (m *mockEKSAPI) DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	key := aws.ToString(params.ClusterName) + "/" + aws.ToString(params.NodegroupName)
	if ng, ok := m.nodeGroupDetails[key]; ok {
		return &eks.DescribeNodegroupOutput{Nodegroup: ng}, nil
	}
	now := time.Now()
	return &eks.DescribeNodegroupOutput{
		Nodegroup: &types.Nodegroup{
			NodegroupName: params.NodegroupName,
			ClusterName:   params.ClusterName,
			Status:        types.NodegroupStatusActive,
			CapacityType:  types.CapacityTypesOnDemand,
			InstanceTypes: []string{"t3.medium"},
			ScalingConfig: &types.NodegroupScalingConfig{
				DesiredSize: aws.Int32(2),
				MinSize:     aws.Int32(1),
				MaxSize:     aws.Int32(5),
			},
			CreatedAt: &now,
		},
	}, nil
}

func (m *mockEKSAPI) ListFargateProfiles(ctx context.Context, params *eks.ListFargateProfilesInput, optFns ...func(*eks.Options)) (*eks.ListFargateProfilesOutput, error) {
	clusterName := aws.ToString(params.ClusterName)
	return &eks.ListFargateProfilesOutput{
		FargateProfileNames: m.fargateProfiles[clusterName],
	}, nil
}

func (m *mockEKSAPI) DescribeFargateProfile(ctx context.Context, params *eks.DescribeFargateProfileInput, optFns ...func(*eks.Options)) (*eks.DescribeFargateProfileOutput, error) {
	key := aws.ToString(params.ClusterName) + "/" + aws.ToString(params.FargateProfileName)
	if fp, ok := m.fargateDetails[key]; ok {
		return &eks.DescribeFargateProfileOutput{FargateProfile: fp}, nil
	}
	now := time.Now()
	return &eks.DescribeFargateProfileOutput{
		FargateProfile: &types.FargateProfile{
			FargateProfileName: params.FargateProfileName,
			ClusterName:        params.ClusterName,
			Status:             types.FargateProfileStatusActive,
			Selectors: []types.FargateProfileSelector{
				{Namespace: aws.String("default")},
			},
			CreatedAt: &now,
		},
	}, nil
}

func TestListClusters(t *testing.T) {
	mock := newMockEKSAPI()
	mock.clusters = []string{"cluster-1", "cluster-2"}
	svc := NewServiceWithAPI(mock)

	clusters, err := svc.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(clusters))
	}
}

func TestGetCluster(t *testing.T) {
	mock := newMockEKSAPI()
	svc := NewServiceWithAPI(mock)

	cluster, err := svc.GetCluster(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.Name != "test-cluster" {
		t.Errorf("expected name 'test-cluster', got '%s'", cluster.Name)
	}
	if cluster.Status != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got '%s'", cluster.Status)
	}
	if cluster.Version != "1.28" {
		t.Errorf("expected version '1.28', got '%s'", cluster.Version)
	}
	if cluster.VPCID != "vpc-12345678" {
		t.Errorf("expected VPC 'vpc-12345678', got '%s'", cluster.VPCID)
	}
	if !cluster.EndpointPublicAccess {
		t.Error("expected EndpointPublicAccess to be true")
	}
}

func TestListNodeGroups(t *testing.T) {
	mock := newMockEKSAPI()
	mock.nodeGroups["test-cluster"] = []string{"ng-1", "ng-2"}
	svc := NewServiceWithAPI(mock)

	nodeGroups, err := svc.ListNodeGroups(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodeGroups) != 2 {
		t.Errorf("expected 2 node groups, got %d", len(nodeGroups))
	}
}

func TestGetNodeGroup(t *testing.T) {
	mock := newMockEKSAPI()
	svc := NewServiceWithAPI(mock)

	ng, err := svc.GetNodeGroup(context.Background(), "test-cluster", "ng-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ng.Name != "ng-1" {
		t.Errorf("expected name 'ng-1', got '%s'", ng.Name)
	}
	if ng.Status != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got '%s'", ng.Status)
	}
	if ng.DesiredSize != 2 {
		t.Errorf("expected desired size 2, got %d", ng.DesiredSize)
	}
}

func TestListFargateProfiles(t *testing.T) {
	mock := newMockEKSAPI()
	mock.fargateProfiles["test-cluster"] = []string{"fp-1"}
	svc := NewServiceWithAPI(mock)

	profiles, err := svc.ListFargateProfiles(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(profiles))
	}
}

func TestGetFargateProfile(t *testing.T) {
	mock := newMockEKSAPI()
	svc := NewServiceWithAPI(mock)

	fp, err := svc.GetFargateProfile(context.Background(), "test-cluster", "fp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.Name != "fp-1" {
		t.Errorf("expected name 'fp-1', got '%s'", fp.Name)
	}
	if fp.Status != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got '%s'", fp.Status)
	}
	if len(fp.Selectors) != 1 {
		t.Errorf("expected 1 selector, got %d", len(fp.Selectors))
	}
}
