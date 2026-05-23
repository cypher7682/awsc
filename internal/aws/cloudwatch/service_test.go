package cloudwatch

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// mockCWClient implements CloudWatchAPI for testing.
type mockCWClient struct {
	output *cloudwatch.GetMetricDataOutput
	err    error
}

func (m *mockCWClient) GetMetricData(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	return m.output, m.err
}

func TestGetEC2Metrics(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockCWClient{
		output: &cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{
					Id:         aws.String("mcpuutilization"),
					Timestamps: []time.Time{now, now.Add(-5 * time.Minute), now.Add(-10 * time.Minute)},
					Values:     []float64{45.2, 38.1, 52.7},
				},
				{
					Id:         aws.String("mnetworkin"),
					Timestamps: []time.Time{now, now.Add(-5 * time.Minute)},
					Values:     []float64{1024000, 512000},
				},
			},
		},
	}

	svc := NewServiceWithClient(mock)
	results, err := svc.GetEC2Metrics(context.Background(), "i-abc123",
		[]EC2MetricName{MetricCPUUtilization, MetricNetworkIn}, 300, 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// CPU - should be reversed (chronological)
	cpu := results[0]
	if cpu.Label != "CPUUtilization" {
		t.Errorf("expected label 'CPUUtilization', got '%s'", cpu.Label)
	}
	if len(cpu.Datapoints) != 3 {
		t.Errorf("expected 3 CPU datapoints, got %d", len(cpu.Datapoints))
	}
	// Oldest first (reversed from CloudWatch response)
	if cpu.Datapoints[0].Value != 52.7 {
		t.Errorf("expected oldest value 52.7, got %.1f", cpu.Datapoints[0].Value)
	}

	// Network
	net := results[1]
	if net.Label != "NetworkIn" {
		t.Errorf("expected label 'NetworkIn', got '%s'", net.Label)
	}
	if net.Unit != "Bytes" {
		t.Errorf("expected unit 'Bytes', got '%s'", net.Unit)
	}
}

func TestGetEC2Metrics_Error(t *testing.T) {
	mock := &mockCWClient{
		err: context.DeadlineExceeded,
	}

	svc := NewServiceWithClient(mock)
	_, err := svc.GetEC2Metrics(context.Background(), "i-abc123",
		[]EC2MetricName{MetricCPUUtilization}, 300, 1*time.Hour)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEC2Metrics_NoData(t *testing.T) {
	mock := &mockCWClient{
		output: &cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{},
		},
	}

	svc := NewServiceWithClient(mock)
	results, err := svc.GetEC2Metrics(context.Background(), "i-abc123",
		[]EC2MetricName{MetricCPUUtilization}, 300, 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Datapoints) != 0 {
		t.Errorf("expected 0 datapoints, got %d", len(results[0].Datapoints))
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CPUUtilization", "mcpuutilization"},
		{"NetworkIn", "mnetworkin"},
		{"StatusCheckFailed", "mstatuscheckfailed"},
	}
	for _, tt := range tests {
		got := sanitizeID(tt.input, 0)
		if got != tt.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultEC2Metrics(t *testing.T) {
	if len(DefaultEC2Metrics) == 0 {
		t.Error("expected default metrics")
	}
	for _, m := range DefaultEC2Metrics {
		if _, ok := MetricUnit[m]; !ok {
			t.Errorf("metric %s missing from MetricUnit", m)
		}
		if _, ok := MetricStatistic[m]; !ok {
			t.Errorf("metric %s missing from MetricStatistic", m)
		}
	}
}
