package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestReleaseContract(t *testing.T) {
	typ := reflect.TypeOf(Release{})
	for _, field := range []string{"ExecutionIntentID", "ApplicationID", "ManifestID", "ImageID", "EnvironmentID", "RoutesSnapshot", "AppConfigSnapshot", "Type", "Status"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Release missing field %s", field)
		}
		if (field == "ImageID" || field == "ManifestID") && f.Type != reflect.TypeOf(uuid.UUID{}) {
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
