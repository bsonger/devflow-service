package domain

import (
	"reflect"
	"testing"

	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
)

func TestReleaseContract(t *testing.T) {
	typ := reflect.TypeOf(Release{})
	for _, field := range []string{"ExecutionIntentID", "ApplicationID", "ManifestID", "EnvironmentID", "RoutesSnapshot", "AppConfigSnapshot", "Type", "Status"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Release missing field %s", field)
		}
		if field == "ManifestID" && f.Type != reflect.TypeOf(uuid.UUID{}) {
			t.Fatalf("Release.%s type = %v, want uuid.UUID", field, f.Type)
		}
	}
}

func TestBaseModelWithCreateDefault(t *testing.T) {
	var base BaseModel
	base.WithCreateDefault()

	if base.ID == uuid.Nil {
		t.Fatal("BaseModel.WithCreateDefault should assign a UUID")
	}
	if base.CreatedAt.IsZero() || base.UpdatedAt.IsZero() {
		t.Fatal("BaseModel.WithCreateDefault should set timestamps")
	}
}

func TestReleaseFrozenWorkloadContractUsesConstrainedTypes(t *testing.T) {
	typ := reflect.TypeOf(ReleaseFrozenWorkload{})
	resourcesField, ok := typ.FieldByName("Resources")
	if !ok {
		t.Fatal("ReleaseFrozenWorkload missing Resources field")
	}
	if resourcesField.Type != reflect.TypeOf(workloadconfigdomain.WorkloadResourceRequirements{}) {
		t.Fatalf("ReleaseFrozenWorkload.Resources type = %v", resourcesField.Type)
	}
	probesField, ok := typ.FieldByName("Probes")
	if !ok {
		t.Fatal("ReleaseFrozenWorkload missing Probes field")
	}
	if probesField.Type != reflect.TypeOf(workloadconfigdomain.WorkloadProbes{}) {
		t.Fatalf("ReleaseFrozenWorkload.Probes type = %v", probesField.Type)
	}
}
