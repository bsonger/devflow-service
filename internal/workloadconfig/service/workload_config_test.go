package service

import (
	"testing"

	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
)

func TestValidateWorkloadConfigAcceptsConstrainedWriteShape(t *testing.T) {
	item := &domain.WorkloadConfig{
		ApplicationID: uuid.New(),
		Replicas:      2,
		Resources: domain.WorkloadResourceRequirements{
			SizeClass: domain.WorkloadSizeClassMedium,
		},
		Probes: domain.WorkloadProbes{
			Liveness: &domain.WorkloadProbe{Path: "/healthz", Port: "http", PeriodSeconds: 10},
		},
		Metrics: domain.WorkloadMetrics{
			Enabled: true,
			Port:    9090,
		},
		Env: []domain.EnvVar{{Name: "LOG_LEVEL", Value: ""}, {Name: "FEATURE_FLAG", Value: "enabled"}},
	}

	if err := validateWorkloadConfig(item); err != nil {
		t.Fatalf("validateWorkloadConfig returned error: %v", err)
	}
	if item.Metrics.ScrapeProfile != domain.WorkloadMetricsScrapeProfileDefault {
		t.Fatalf("metrics.scrape_profile = %q, want %q", item.Metrics.ScrapeProfile, domain.WorkloadMetricsScrapeProfileDefault)
	}
}

func TestValidateWorkloadConfigRejectsInvalidContractDrift(t *testing.T) {
	item := &domain.WorkloadConfig{
		Replicas: -1,
		Resources: domain.WorkloadResourceRequirements{
			SizeClass: "jumbo",
			Requests:  domain.WorkloadResourceList{CPU: "999m", Memory: "999Mi"},
		},
		Probes: domain.WorkloadProbes{
			Readiness: &domain.WorkloadProbe{Path: "readyz", Port: ""},
		},
		Metrics: domain.WorkloadMetrics{
			Enabled:       true,
			Port:          0,
			ScrapeProfile: "burst",
		},
		Env: []domain.EnvVar{{Name: " "}, {Name: "LOG_LEVEL", Value: "info"}, {Name: "LOG_LEVEL", Value: "debug"}},
	}

	err := validateWorkloadConfig(item)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument error, got %T %v", err, err)
	}
	message := err.Error()
	for _, want := range []string{
		"application_id is required",
		"replicas must be >= 0",
		"resources.size_class must be one of:",
		"resources.requests must not be provided on write",
		"probes.readiness.path must start with '/'",
		"probes.readiness.port is required when probes.readiness.path is set",
		"metrics.port must be > 0 when metrics.enabled is true",
		"metrics.scrape_profile must be one of:",
		"env[0].name is required",
		"env[2].name duplicates env[1].name \"LOG_LEVEL\"",
	} {
		if !contains(message, want) {
			t.Fatalf("expected error to contain %q, got %q", want, message)
		}
	}
}

func TestValidateWorkloadConfigRejectsLegacyWideWriteFields(t *testing.T) {
	item := &domain.WorkloadConfig{
		ApplicationID: uuid.New(),
		Replicas:      1,
		Resources: domain.WorkloadResourceRequirements{
			SizeClass: domain.WorkloadSizeClassSmall,
			Requests:  domain.WorkloadResourceList{CPU: "100m", Memory: "64Mi"},
			Limits:    domain.WorkloadResourceList{CPU: "500m", Memory: "512Mi"},
		},
	}

	err := validateWorkloadConfig(item)
	if err == nil {
		t.Fatal("expected validation error")
	}
	message := err.Error()
	if !contains(message, "resources.requests must not be provided on write") {
		t.Fatalf("expected requests rejection, got %q", message)
	}
	if !contains(message, "resources.limits must not be provided on write") {
		t.Fatalf("expected limits rejection, got %q", message)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
