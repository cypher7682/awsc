// Package cloudwatch provides the CloudWatch service layer for awsc.
package cloudwatch

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// MetricDatapoint represents a single metric data point.
type MetricDatapoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricResult holds the result of a metric query.
type MetricResult struct {
	Label      string
	Unit       string
	Datapoints []MetricDatapoint
}

// EC2MetricName identifies a standard EC2 CloudWatch metric.
type EC2MetricName string

const (
	MetricCPUUtilization    EC2MetricName = "CPUUtilization"
	MetricNetworkIn         EC2MetricName = "NetworkIn"
	MetricNetworkOut        EC2MetricName = "NetworkOut"
	MetricDiskReadBytes     EC2MetricName = "DiskReadBytes"
	MetricDiskWriteBytes    EC2MetricName = "DiskWriteBytes"
	MetricDiskReadOps       EC2MetricName = "DiskReadOps"
	MetricDiskWriteOps      EC2MetricName = "DiskWriteOps"
	MetricStatusCheckFailed EC2MetricName = "StatusCheckFailed"
)

// DefaultEC2Metrics is the set of metrics shown on the EC2 Monitoring tab.
var DefaultEC2Metrics = []EC2MetricName{
	MetricCPUUtilization,
	MetricNetworkIn,
	MetricNetworkOut,
	MetricDiskReadBytes,
	MetricDiskWriteBytes,
	MetricStatusCheckFailed,
}

// MetricUnit maps metric names to their expected units.
var MetricUnit = map[EC2MetricName]string{
	MetricCPUUtilization:    "Percent",
	MetricNetworkIn:         "Bytes",
	MetricNetworkOut:        "Bytes",
	MetricDiskReadBytes:     "Bytes",
	MetricDiskWriteBytes:    "Bytes",
	MetricDiskReadOps:       "Count",
	MetricDiskWriteOps:      "Count",
	MetricStatusCheckFailed: "Count",
}

// MetricStatistic maps metric names to the statistic we request.
var MetricStatistic = map[EC2MetricName]types.Statistic{
	MetricCPUUtilization:    types.StatisticAverage,
	MetricNetworkIn:         types.StatisticSum,
	MetricNetworkOut:        types.StatisticSum,
	MetricDiskReadBytes:     types.StatisticSum,
	MetricDiskWriteBytes:    types.StatisticSum,
	MetricDiskReadOps:       types.StatisticSum,
	MetricDiskWriteOps:      types.StatisticSum,
	MetricStatusCheckFailed: types.StatisticMaximum,
}

// CloudWatchAPI defines the interface for CloudWatch operations.
type CloudWatchAPI interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// Service wraps the CloudWatch SDK client.
type Service struct {
	client CloudWatchAPI
}

// NewService creates a new CloudWatch service from an AWS config.
func NewService(cfg aws.Config) *Service {
	return &Service{
		client: cloudwatch.NewFromConfig(cfg),
	}
}

// NewServiceFromClient creates a new CloudWatch service from a pre-built SDK client.
func NewServiceFromClient(client *cloudwatch.Client) *Service {
	return &Service{client: client}
}

// NewServiceWithClient creates a new service with a custom client (for testing).
func NewServiceWithClient(client CloudWatchAPI) *Service {
	return &Service{client: client}
}

// GetEC2Metrics fetches multiple CloudWatch metrics for an EC2 instance.
// period is the granularity in seconds (e.g. 300 = 5 min).
// duration is how far back to look (e.g. 3*time.Hour).
func (s *Service) GetEC2Metrics(ctx context.Context, instanceID string, metrics []EC2MetricName, period int32, duration time.Duration) ([]MetricResult, error) {
	now := time.Now().UTC()
	startTime := now.Add(-duration)

	// Build metric data queries
	queries := make([]types.MetricDataQuery, len(metrics))
	for i, m := range metrics {
		stat := MetricStatistic[m]
		if stat == "" {
			stat = types.StatisticAverage
		}
		queries[i] = types.MetricDataQuery{
			Id: aws.String(sanitizeID(string(m), i)),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Namespace:  aws.String("AWS/EC2"),
					MetricName: aws.String(string(m)),
					Dimensions: []types.Dimension{
						{
							Name:  aws.String("InstanceId"),
							Value: aws.String(instanceID),
						},
					},
				},
				Period: aws.Int32(period),
				Stat:   aws.String(string(stat)),
			},
		}
	}

	input := &cloudwatch.GetMetricDataInput{
		StartTime:         &startTime,
		EndTime:           &now,
		MetricDataQueries: queries,
	}

	output, err := s.client.GetMetricData(ctx, input)
	if err != nil {
		return nil, err
	}

	// Build results, matching query IDs back to metric names
	results := make([]MetricResult, len(metrics))
	for i, m := range metrics {
		results[i] = MetricResult{
			Label: string(m),
			Unit:  MetricUnit[m],
		}
		qID := sanitizeID(string(m), i)
		for _, r := range output.MetricDataResults {
			if r.Id != nil && *r.Id == qID {
				// CloudWatch returns newest-first; we want chronological order
				for j := len(r.Timestamps) - 1; j >= 0; j-- {
					results[i].Datapoints = append(results[i].Datapoints, MetricDatapoint{
						Timestamp: r.Timestamps[j],
						Value:     r.Values[j],
					})
				}
				break
			}
		}
	}

	return results, nil
}

// sanitizeID creates a valid CloudWatch metric data query ID.
// IDs must start with a lowercase letter and contain only [a-z0-9_].
func sanitizeID(name string, idx int) string {
	var b []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			b = append(b, c)
		} else if c >= 'A' && c <= 'Z' {
			b = append(b, c+32) // lowercase
		}
	}
	// Prefix with 'm' to ensure it starts with a letter, add index for uniqueness
	return "m" + string(b)
}
