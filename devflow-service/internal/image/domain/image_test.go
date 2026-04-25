package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestImageContract(t *testing.T) {
	typ := reflect.TypeOf(Image{})
	for _, field := range []string{"ExecutionIntentID", "ApplicationID", "ConfigurationRevisionID", "RuntimeSpecRevisionID", "Name", "Tag", "Branch", "RepoAddress", "PipelineID", "Status"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Image missing field %s", field)
		}
		if field == "ApplicationID" && f.Type != reflect.TypeOf(uuid.UUID{}) {
			t.Fatalf("Image.ApplicationID type = %v, want uuid.UUID", f.Type)
		}
	}
	for _, removed := range []string{"ApplicationName", "GitRepo", "ConfigMaps", "Service", "Internet", "Envs", "Replica", "Type", "Services"} {
		if _, ok := typ.FieldByName(removed); ok {
			t.Fatalf("Image should not expose legacy field %s", removed)
		}
	}
}
