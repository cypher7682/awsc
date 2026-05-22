package ecr

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// mockECRClient implements ECRAPI for testing.
type mockECRClient struct {
	describeRepositoriesOutput *ecr.DescribeRepositoriesOutput
	describeRepositoriesError  error
	describeImagesOutput       *ecr.DescribeImagesOutput
	describeImagesError        error
	deleteRepositoryOutput     *ecr.DeleteRepositoryOutput
	deleteRepositoryError      error
	batchDeleteImageOutput     *ecr.BatchDeleteImageOutput
	batchDeleteImageError      error
	createRepositoryOutput     *ecr.CreateRepositoryOutput
	createRepositoryError      error
}

func (m *mockECRClient) DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return m.describeRepositoriesOutput, m.describeRepositoriesError
}

func (m *mockECRClient) DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	return m.describeImagesOutput, m.describeImagesError
}

func (m *mockECRClient) DeleteRepository(ctx context.Context, params *ecr.DeleteRepositoryInput, optFns ...func(*ecr.Options)) (*ecr.DeleteRepositoryOutput, error) {
	return m.deleteRepositoryOutput, m.deleteRepositoryError
}

func (m *mockECRClient) BatchDeleteImage(ctx context.Context, params *ecr.BatchDeleteImageInput, optFns ...func(*ecr.Options)) (*ecr.BatchDeleteImageOutput, error) {
	return m.batchDeleteImageOutput, m.batchDeleteImageError
}

func (m *mockECRClient) CreateRepository(ctx context.Context, params *ecr.CreateRepositoryInput, optFns ...func(*ecr.Options)) (*ecr.CreateRepositoryOutput, error) {
	return m.createRepositoryOutput, m.createRepositoryError
}

func TestListRepositories(t *testing.T) {
	now := time.Now()
	mock := &mockECRClient{
		describeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
			Repositories: []types.Repository{
				{
					RepositoryName: aws.String("my-app"),
					RepositoryUri:  aws.String("123456789.dkr.ecr.us-east-1.amazonaws.com/my-app"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789:repository/my-app"),
					CreatedAt:      &now,
					ImageTagMutability: types.ImageTagMutabilityMutable,
					ImageScanningConfiguration: &types.ImageScanningConfiguration{
						ScanOnPush: true,
					},
				},
				{
					RepositoryName: aws.String("my-api"),
					RepositoryUri:  aws.String("123456789.dkr.ecr.us-east-1.amazonaws.com/my-api"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789:repository/my-api"),
					CreatedAt:      &now,
					ImageTagMutability: types.ImageTagMutabilityImmutable,
					ImageScanningConfiguration: &types.ImageScanningConfiguration{
						ScanOnPush: false,
					},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	repos, err := svc.ListRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	if repos[0].Name != "my-app" {
		t.Errorf("expected 'my-app', got '%s'", repos[0].Name)
	}
	if !repos[0].ScanOnPush {
		t.Error("expected ScanOnPush to be true for my-app")
	}
	if repos[0].MutabilityTag != "MUTABLE" {
		t.Errorf("expected 'MUTABLE', got '%s'", repos[0].MutabilityTag)
	}

	if repos[1].Name != "my-api" {
		t.Errorf("expected 'my-api', got '%s'", repos[1].Name)
	}
	if repos[1].MutabilityTag != "IMMUTABLE" {
		t.Errorf("expected 'IMMUTABLE', got '%s'", repos[1].MutabilityTag)
	}
}

func TestGetRepository(t *testing.T) {
	mock := &mockECRClient{
		describeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
			Repositories: []types.Repository{
				{
					RepositoryName:     aws.String("my-app"),
					RepositoryUri:      aws.String("123456789.dkr.ecr.us-east-1.amazonaws.com/my-app"),
					RepositoryArn:      aws.String("arn:aws:ecr:us-east-1:123456789:repository/my-app"),
					ImageTagMutability: types.ImageTagMutabilityMutable,
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	repo, err := svc.GetRepository(context.Background(), "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "my-app" {
		t.Errorf("expected 'my-app', got '%s'", repo.Name)
	}
}

func TestGetRepository_NotFound(t *testing.T) {
	mock := &mockECRClient{
		describeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
			Repositories: []types.Repository{},
		},
	}

	svc := NewServiceWithClient(mock)
	_, err := svc.GetRepository(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent repo")
	}
}

func TestListImages(t *testing.T) {
	now := time.Now()
	mock := &mockECRClient{
		describeImagesOutput: &ecr.DescribeImagesOutput{
			ImageDetails: []types.ImageDetail{
				{
					ImageDigest:      aws.String("sha256:abc123"),
					ImageTags:        []string{"latest", "v1.2.3"},
					ImagePushedAt:    &now,
					ImageSizeInBytes: aws.Int64(52428800),
					ImageScanStatus: &types.ImageScanStatus{
						Status: types.ScanStatusComplete,
					},
				},
				{
					ImageDigest:      aws.String("sha256:def456"),
					ImageTags:        []string{"v1.2.2"},
					ImagePushedAt:    &now,
					ImageSizeInBytes: aws.Int64(48234567),
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	images, err := svc.ListImages(context.Background(), "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	if images[0].Digest != "sha256:abc123" {
		t.Errorf("expected digest 'sha256:abc123', got '%s'", images[0].Digest)
	}
	if len(images[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(images[0].Tags))
	}
	if images[0].SizeBytes != 52428800 {
		t.Errorf("expected size 52428800, got %d", images[0].SizeBytes)
	}
	if images[0].ScanStatus != "COMPLETE" {
		t.Errorf("expected scan status 'COMPLETE', got '%s'", images[0].ScanStatus)
	}
}

func TestDeleteImage(t *testing.T) {
	mock := &mockECRClient{
		batchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
			ImageIds: []types.ImageIdentifier{
				{ImageDigest: aws.String("sha256:abc123")},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	err := svc.DeleteImage(context.Background(), "my-app", "sha256:abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteImage_Failure(t *testing.T) {
	mock := &mockECRClient{
		batchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
			Failures: []types.ImageFailure{
				{
					FailureReason: aws.String("ImageNotFound"),
					FailureCode:   types.ImageFailureCodeImageNotFound,
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	err := svc.DeleteImage(context.Background(), "my-app", "sha256:nonexistent")
	if err == nil {
		t.Error("expected error for failed deletion")
	}
}

func TestDeleteRepository(t *testing.T) {
	mock := &mockECRClient{
		deleteRepositoryOutput: &ecr.DeleteRepositoryOutput{},
	}

	svc := NewServiceWithClient(mock)
	err := svc.DeleteRepository(context.Background(), "my-app", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepository(t *testing.T) {
	now := time.Now()
	mock := &mockECRClient{
		createRepositoryOutput: &ecr.CreateRepositoryOutput{
			Repository: &types.Repository{
				RepositoryName:     aws.String("new-repo"),
				RepositoryUri:      aws.String("123456789.dkr.ecr.us-east-1.amazonaws.com/new-repo"),
				RepositoryArn:      aws.String("arn:aws:ecr:us-east-1:123456789:repository/new-repo"),
				CreatedAt:          &now,
				ImageTagMutability: types.ImageTagMutabilityImmutable,
				ImageScanningConfiguration: &types.ImageScanningConfiguration{
					ScanOnPush: true,
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	repo, err := svc.CreateRepository(context.Background(), "new-repo", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "new-repo" {
		t.Errorf("expected 'new-repo', got '%s'", repo.Name)
	}
	if !repo.ScanOnPush {
		t.Error("expected ScanOnPush to be true")
	}
	if repo.MutabilityTag != "IMMUTABLE" {
		t.Errorf("expected 'IMMUTABLE', got '%s'", repo.MutabilityTag)
	}
}
