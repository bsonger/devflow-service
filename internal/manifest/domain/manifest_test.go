package domain

import (
	"reflect"
	"testing"
)

func TestManifestContract(t *testing.T) {
	typ := reflect.TypeOf(Manifest{})
	for _, field := range []string{
		"ApplicationID",
		"EnvironmentID",
		"ImageID",
		"ImageRef",
		"ArtifactRepository",
		"ArtifactTag",
		"ArtifactRef",
		"ArtifactDigest",
		"ArtifactMediaType",
		"ArtifactPushedAt",
		"ServicesSnapshot",
		"WorkloadConfigSnapshot",
		"RenderedObjects",
		"RenderedYAML",
		"Status",
	} {
		if _, ok := typ.FieldByName(field); !ok {
			t.Fatalf("Manifest missing field %s", field)
		}
	}
}
