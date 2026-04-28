package domain

import (
	"reflect"
	"testing"
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
}
