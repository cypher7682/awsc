// Package secretsmanager provides AWS Secrets Manager operations.
package secretsmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// Secret represents a secret in AWS Secrets Manager.
type Secret struct {
	Name              string
	ARN               string
	Description       string
	KmsKeyID          string
	RotationEnabled   bool
	RotationLambdaARN string
	RotationSchedule  string
	LastAccessedDate  *time.Time
	LastChangedDate   *time.Time
	LastRetrievedDate *time.Time
	CreatedDate       *time.Time
	DeletedDate       *time.Time
	PrimaryRegion     string
	Replicas          []ReplicaRegion
	Versions          []SecretVersion
	Tags              map[string]string
	// Owning service (if managed by another AWS service)
	OwningService string
}

// ReplicaRegion represents a replica of a secret in another region.
type ReplicaRegion struct {
	Region           string
	KmsKeyID         string
	Status           string
	StatusMessage    string
	LastAccessedDate *time.Time
}

// SecretVersion represents a version of a secret.
type SecretVersion struct {
	VersionID     string
	StagingLabels []string
	CreatedDate   *time.Time
	LastAccessedDate *time.Time
}

// SecretValue holds the retrieved secret value.
type SecretValue struct {
	Name          string
	ARN           string
	VersionID     string
	VersionStages []string
	SecretString  string
	SecretBinary  []byte
	CreatedDate   *time.Time
}

// SecretsManagerAPI defines the interface for Secrets Manager operations.
type SecretsManagerAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	DescribeSecret(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Service provides Secrets Manager operations.
type Service struct {
	client SecretsManagerAPI
}

// NewService creates a new Secrets Manager service with the given AWS config.
func NewService(cfg aws.Config) *Service {
	return &Service{
		client: secretsmanager.NewFromConfig(cfg),
	}
}

// NewServiceFromClient creates a new service from an existing client.
func NewServiceFromClient(client SecretsManagerAPI) *Service {
	return &Service{client: client}
}

// ListSecrets returns all secrets in the account.
func (s *Service) ListSecrets(ctx context.Context) ([]Secret, error) {
	var secrets []Secret
	paginator := secretsmanager.NewListSecretsPaginator(s.client.(*secretsmanager.Client), &secretsmanager.ListSecretsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, entry := range page.SecretList {
			secrets = append(secrets, secretEntryToSecret(entry))
		}
	}

	return secrets, nil
}

// ListSecretsWithClient returns all secrets using any client (for testing).
func (s *Service) ListSecretsWithClient(ctx context.Context) ([]Secret, error) {
	var secrets []Secret
	var nextToken *string

	for {
		output, err := s.client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, entry := range output.SecretList {
			secrets = append(secrets, secretEntryToSecret(entry))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return secrets, nil
}

// DescribeSecret returns detailed information about a secret.
func (s *Service) DescribeSecret(ctx context.Context, secretID string) (*Secret, error) {
	output, err := s.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretID),
	})
	if err != nil {
		return nil, err
	}

	secret := &Secret{
		Name:              aws.ToString(output.Name),
		ARN:               aws.ToString(output.ARN),
		Description:       aws.ToString(output.Description),
		KmsKeyID:          aws.ToString(output.KmsKeyId),
		RotationEnabled:   aws.ToBool(output.RotationEnabled),
		LastAccessedDate:  output.LastAccessedDate,
		LastChangedDate:   output.LastChangedDate,
		CreatedDate:       output.CreatedDate,
		DeletedDate:       output.DeletedDate,
		PrimaryRegion:     aws.ToString(output.PrimaryRegion),
		OwningService:     aws.ToString(output.OwningService),
		Tags:              make(map[string]string),
	}

	// Rotation configuration
	if output.RotationRules != nil {
		if output.RotationRules.ScheduleExpression != nil {
			secret.RotationSchedule = aws.ToString(output.RotationRules.ScheduleExpression)
		} else if output.RotationRules.AutomaticallyAfterDays != nil {
			secret.RotationSchedule = formatDays(*output.RotationRules.AutomaticallyAfterDays)
		}
	}
	if output.RotationLambdaARN != nil {
		secret.RotationLambdaARN = aws.ToString(output.RotationLambdaARN)
	}

	// Replicas
	for _, r := range output.ReplicationStatus {
		secret.Replicas = append(secret.Replicas, ReplicaRegion{
			Region:           aws.ToString(r.Region),
			KmsKeyID:         aws.ToString(r.KmsKeyId),
			Status:           string(r.Status),
			StatusMessage:    aws.ToString(r.StatusMessage),
			LastAccessedDate: r.LastAccessedDate,
		})
	}

	// Versions
	for versionID, stages := range output.VersionIdsToStages {
		secret.Versions = append(secret.Versions, SecretVersion{
			VersionID:     versionID,
			StagingLabels: stages,
		})
	}

	// Tags
	for _, tag := range output.Tags {
		if tag.Key != nil && tag.Value != nil {
			secret.Tags[*tag.Key] = *tag.Value
		}
	}

	return secret, nil
}

// GetSecretValue retrieves the secret value.
func (s *Service) GetSecretValue(ctx context.Context, secretID string, versionID, versionStage *string) (*SecretValue, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	}
	if versionID != nil {
		input.VersionId = versionID
	}
	if versionStage != nil {
		input.VersionStage = versionStage
	}

	output, err := s.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, err
	}

	return &SecretValue{
		Name:          aws.ToString(output.Name),
		ARN:           aws.ToString(output.ARN),
		VersionID:     aws.ToString(output.VersionId),
		VersionStages: output.VersionStages,
		SecretString:  aws.ToString(output.SecretString),
		SecretBinary:  output.SecretBinary,
		CreatedDate:   output.CreatedDate,
	}, nil
}

// secretEntryToSecret converts a SecretListEntry to our Secret type.
func secretEntryToSecret(entry types.SecretListEntry) Secret {
	secret := Secret{
		Name:             aws.ToString(entry.Name),
		ARN:              aws.ToString(entry.ARN),
		Description:      aws.ToString(entry.Description),
		KmsKeyID:         aws.ToString(entry.KmsKeyId),
		RotationEnabled:  aws.ToBool(entry.RotationEnabled),
		LastAccessedDate: entry.LastAccessedDate,
		LastChangedDate:  entry.LastChangedDate,
		CreatedDate:      entry.CreatedDate,
		DeletedDate:      entry.DeletedDate,
		PrimaryRegion:    aws.ToString(entry.PrimaryRegion),
		OwningService:    aws.ToString(entry.OwningService),
		Tags:             make(map[string]string),
	}

	// Rotation configuration
	if entry.RotationRules != nil {
		if entry.RotationRules.ScheduleExpression != nil {
			secret.RotationSchedule = aws.ToString(entry.RotationRules.ScheduleExpression)
		} else if entry.RotationRules.AutomaticallyAfterDays != nil {
			secret.RotationSchedule = formatDays(*entry.RotationRules.AutomaticallyAfterDays)
		}
	}
	if entry.RotationLambdaARN != nil {
		secret.RotationLambdaARN = aws.ToString(entry.RotationLambdaARN)
	}

	// Tags
	for _, tag := range entry.Tags {
		if tag.Key != nil && tag.Value != nil {
			secret.Tags[*tag.Key] = *tag.Value
		}
	}

	return secret
}

// formatDays formats a number of days as a human-readable string.
func formatDays(days int64) string {
	if days == 1 {
		return "every day"
	}
	return fmt.Sprintf("every %d days", days)
}
