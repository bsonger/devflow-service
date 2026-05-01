package domain

import (
	"reflect"
	"testing"

	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
)

func TestManifestContract(t *testing.T) {
	typ := reflect.TypeOf(Manifest{})
	for _, field := range []string{
		"ApplicationID",
		"GitRevision",
		"RepoAddress",
		"CommitHash",
		"ImageTag",
		"ImageDigest",
		"PipelineID",
		"TraceID",
		"SpanID",
		"Steps",
		"ImageRef",
		"ServicesSnapshot",
		"WorkloadConfigSnapshot",
		"Status",
	} {
		if _, ok := typ.FieldByName(field); !ok {
			t.Fatalf("Manifest missing field %s", field)
		}
	}

	workloadField, ok := reflect.TypeOf(ManifestWorkloadConfig{}).FieldByName("Resources")
	if !ok {
		t.Fatal("ManifestWorkloadConfig missing Resources field")
	}
	if workloadField.Type != reflect.TypeOf(workloadconfigdomain.WorkloadResourceRequirements{}) {
		t.Fatalf("ManifestWorkloadConfig.Resources type = %v", workloadField.Type)
	}
	probesField, ok := reflect.TypeOf(ManifestWorkloadConfig{}).FieldByName("Probes")
	if !ok {
		t.Fatal("ManifestWorkloadConfig missing Probes field")
	}
	if probesField.Type != reflect.TypeOf(workloadconfigdomain.WorkloadProbes{}) {
		t.Fatalf("ManifestWorkloadConfig.Probes type = %v", probesField.Type)
	}
}
