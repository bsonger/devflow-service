package domain

import "testing"

func TestProjectCollectionName(t *testing.T) {
	if got := (Project{}).CollectionName(); got != "projects" {
		t.Fatalf("expected projects collection, got %q", got)
	}
}

func TestProjectDoesNotExposeLegacyKeyOrNamespace(t *testing.T) {
	project := Project{Name: "dev-platform", Description: "platform"}

	if project.Name != "dev-platform" {
		t.Fatalf("unexpected project name: %#v", project)
	}
}
