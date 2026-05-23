// Package ecr provides the ECR service layer for awsc.
package ecr

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// Repository represents a simplified ECR repository.
type Repository struct {
	Name         string
	URI          string
	ARN          string
	CreatedAt    time.Time
	ImageCount   int
	ScanOnPush   bool
	MutabilityTag string // MUTABLE or IMMUTABLE
}

// Image represents a simplified ECR image.
type Image struct {
	Digest     string
	Tags       []string
	PushedAt   time.Time
	SizeBytes  int64
	ScanStatus string
}

// ECRAPI defines the interface for ECR operations (for testability).
type ECRAPI interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
	DeleteRepository(ctx context.Context, params *ecr.DeleteRepositoryInput, optFns ...func(*ecr.Options)) (*ecr.DeleteRepositoryOutput, error)
	BatchDeleteImage(ctx context.Context, params *ecr.BatchDeleteImageInput, optFns ...func(*ecr.Options)) (*ecr.BatchDeleteImageOutput, error)
	CreateRepository(ctx context.Context, params *ecr.CreateRepositoryInput, optFns ...func(*ecr.Options)) (*ecr.CreateRepositoryOutput, error)
}

// Service provides ECR operations.
type Service struct {
	client ECRAPI
}

// NewService creates a new ECR service from an AWS config.
func NewService(cfg aws.Config) *Service {
	return &Service{
		client: ecr.NewFromConfig(cfg),
	}
}

// NewServiceFromClient creates a new ECR service from a pre-built SDK client.
func NewServiceFromClient(client *ecr.Client) *Service {
	return &Service{client: client}
}

// NewServiceWithClient creates a new ECR service with a custom client (for testing).
func NewServiceWithClient(client ECRAPI) *Service {
	return &Service{client: client}
}

// ListRepositories returns all ECR repositories.
func (s *Service) ListRepositories(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	var nextToken *string

	for {
		input := &ecr.DescribeRepositoriesInput{
			NextToken: nextToken,
		}
		output, err := s.client.DescribeRepositories(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing repositories: %w", err)
		}

		for _, repo := range output.Repositories {
			repos = append(repos, repositoryFromAWS(repo))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return repos, nil
}

// GetRepository returns a single repository by name.
func (s *Service) GetRepository(ctx context.Context, name string) (*Repository, error) {
	input := &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{name},
	}
	output, err := s.client.DescribeRepositories(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing repository %s: %w", name, err)
	}
	if len(output.Repositories) == 0 {
		return nil, fmt.Errorf("repository %s not found", name)
	}
	repo := repositoryFromAWS(output.Repositories[0])
	return &repo, nil
}

// ListImages returns all images in a repository.
func (s *Service) ListImages(ctx context.Context, repoName string) ([]Image, error) {
	var images []Image
	var nextToken *string

	for {
		input := &ecr.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
			NextToken:      nextToken,
		}
		output, err := s.client.DescribeImages(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing images in %s: %w", repoName, err)
		}

		for _, img := range output.ImageDetails {
			images = append(images, imageFromAWS(img))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return images, nil
}

// DeleteImage deletes an image from a repository by digest.
func (s *Service) DeleteImage(ctx context.Context, repoName, digest string) error {
	input := &ecr.BatchDeleteImageInput{
		RepositoryName: aws.String(repoName),
		ImageIds: []types.ImageIdentifier{
			{ImageDigest: aws.String(digest)},
		},
	}
	output, err := s.client.BatchDeleteImage(ctx, input)
	if err != nil {
		return fmt.Errorf("deleting image %s from %s: %w", digest, repoName, err)
	}
	if len(output.Failures) > 0 {
		return fmt.Errorf("failed to delete image: %s", aws.ToString(output.Failures[0].FailureReason))
	}
	return nil
}

// DeleteRepository deletes an ECR repository.
func (s *Service) DeleteRepository(ctx context.Context, name string, force bool) error {
	input := &ecr.DeleteRepositoryInput{
		RepositoryName: aws.String(name),
		Force:          force,
	}
	_, err := s.client.DeleteRepository(ctx, input)
	if err != nil {
		return fmt.Errorf("deleting repository %s: %w", name, err)
	}
	return nil
}

// CreateRepository creates a new ECR repository.
func (s *Service) CreateRepository(ctx context.Context, name string, scanOnPush bool, immutable bool) (*Repository, error) {
	mutability := types.ImageTagMutabilityMutable
	if immutable {
		mutability = types.ImageTagMutabilityImmutable
	}

	input := &ecr.CreateRepositoryInput{
		RepositoryName:     aws.String(name),
		ImageTagMutability: mutability,
	}
	if scanOnPush {
		input.ImageScanningConfiguration = &types.ImageScanningConfiguration{
			ScanOnPush: true,
		}
	}

	output, err := s.client.CreateRepository(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating repository %s: %w", name, err)
	}
	repo := repositoryFromAWS(*output.Repository)
	return &repo, nil
}

// repositoryFromAWS converts an AWS ECR repository to our internal representation.
func repositoryFromAWS(repo types.Repository) Repository {
	r := Repository{
		Name:          aws.ToString(repo.RepositoryName),
		URI:           aws.ToString(repo.RepositoryUri),
		ARN:           aws.ToString(repo.RepositoryArn),
		MutabilityTag: string(repo.ImageTagMutability),
	}
	if repo.CreatedAt != nil {
		r.CreatedAt = *repo.CreatedAt
	}
	if repo.ImageScanningConfiguration != nil {
		r.ScanOnPush = repo.ImageScanningConfiguration.ScanOnPush
	}
	return r
}

// imageFromAWS converts an AWS ECR image detail to our internal representation.
func imageFromAWS(img types.ImageDetail) Image {
	i := Image{
		Digest: aws.ToString(img.ImageDigest),
		Tags:   img.ImageTags,
	}
	if img.ImagePushedAt != nil {
		i.PushedAt = *img.ImagePushedAt
	}
	if img.ImageSizeInBytes != nil {
		i.SizeBytes = *img.ImageSizeInBytes
	}
	if img.ImageScanStatus != nil {
		i.ScanStatus = string(img.ImageScanStatus.Status)
	}
	return i
}
