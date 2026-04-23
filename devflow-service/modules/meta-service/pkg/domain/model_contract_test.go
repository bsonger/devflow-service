package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestApplicationContract(t *testing.T) {
	typ := reflect.TypeOf(Application{})

	for _, field := range []string{"ProjectID", "Name", "RepoAddress", "Description", "ActiveImageID", "Labels"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Application missing field %s", field)
		}
		switch field {
		case "ProjectID":
			if f.Type != reflect.TypeOf(uuid.UUID{}) {
				t.Fatalf("Application.ProjectID type = %v, want uuid.UUID", f.Type)
			}
		case "RepoAddress":
			if got := f.Tag.Get("db"); got != "repo_address" {
				t.Fatalf("Application.RepoAddress db tag = %q, want %q", got, "repo_address")
			}
		case "Labels":
			if f.Type != reflect.TypeOf([]LabelItem{}) {
				t.Fatalf("Application.Labels type = %v, want []LabelItem", f.Type)
			}
		}
	}
}

func TestServiceResourceContract(t *testing.T) {
	typ := reflect.TypeOf(ServiceResource{})
	for _, field := range []string{"ApplicationID", "Name", "Description", "Labels", "Ports"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("ServiceResource missing field %s", field)
		}
		if field == "ApplicationID" && f.Type != reflect.TypeOf(uuid.UUID{}) {
			t.Fatalf("ServiceResource.ApplicationID type = %v, want uuid.UUID", f.Type)
		}
	}
}

func TestProjectContractAfterAudit(t *testing.T) {
	typ := reflect.TypeOf(Project{})
	if _, ok := typ.FieldByName("Status"); ok {
		t.Fatal("Project should not expose Status")
	}
	if f, ok := typ.FieldByName("Labels"); !ok || f.Type != reflect.TypeOf([]LabelItem{}) {
		t.Fatalf("Project.Labels should be []LabelItem, got %#v", f.Type)
	}
}

func TestClusterContract(t *testing.T) {
	typ := reflect.TypeOf(Cluster{})
	for _, field := range []string{"Name", "Server", "KubeConfig", "ArgoCDClusterName", "Description", "Labels", "OnboardingReady", "OnboardingError", "OnboardingCheckedAt"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Cluster missing field %s", field)
		}
		switch field {
		case "Server":
			if got := f.Tag.Get("db"); got != "server" {
				t.Fatalf("Cluster.Server db tag = %q, want %q", got, "server")
			}
		case "KubeConfig":
			if got := f.Tag.Get("db"); got != "kubeconfig" {
				t.Fatalf("Cluster.KubeConfig db tag = %q, want %q", got, "kubeconfig")
			}
		case "ArgoCDClusterName":
			if got := f.Tag.Get("db"); got != "argocd_cluster_name" {
				t.Fatalf("Cluster.ArgoCDClusterName db tag = %q, want %q", got, "argocd_cluster_name")
			}
		case "OnboardingReady":
			if got := f.Tag.Get("db"); got != "onboarding_ready" {
				t.Fatalf("Cluster.OnboardingReady db tag = %q, want %q", got, "onboarding_ready")
			}
			if f.Type.Kind() != reflect.Bool {
				t.Fatalf("Cluster.OnboardingReady type = %v, want bool", f.Type)
			}
		case "OnboardingError":
			if got := f.Tag.Get("db"); got != "onboarding_error" {
				t.Fatalf("Cluster.OnboardingError db tag = %q, want %q", got, "onboarding_error")
			}
		case "OnboardingCheckedAt":
			if got := f.Tag.Get("db"); got != "onboarding_checked_at" {
				t.Fatalf("Cluster.OnboardingCheckedAt db tag = %q, want %q", got, "onboarding_checked_at")
			}
		case "Labels":
			if f.Type != reflect.TypeOf([]LabelItem{}) {
				t.Fatalf("Cluster.Labels type = %v, want []LabelItem", f.Type)
			}
		}
	}
}

func TestEnvironmentContract(t *testing.T) {
	typ := reflect.TypeOf(Environment{})
	for _, field := range []string{"Name", "ClusterID", "Description", "Labels"} {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("Environment missing field %s", field)
		}
		switch field {
		case "ClusterID":
			if f.Type != reflect.TypeOf(uuid.UUID{}) {
				t.Fatalf("Environment.ClusterID type = %v, want uuid.UUID", f.Type)
			}
			if got := f.Tag.Get("db"); got != "cluster_id" {
				t.Fatalf("Environment.ClusterID db tag = %q, want %q", got, "cluster_id")
			}
		case "Labels":
			if f.Type != reflect.TypeOf([]LabelItem{}) {
				t.Fatalf("Environment.Labels type = %v, want []LabelItem", f.Type)
			}
		}
	}
	if _, ok := typ.FieldByName("Cluster"); ok {
		t.Fatal("Environment should not expose Cluster string field")
	}
	if _, ok := typ.FieldByName("Namespace"); ok {
		t.Fatal("Environment should not expose Namespace")
	}
}

func TestBaseModelWithCreateDefault(t *testing.T) {
	var base BaseModel
	base.WithCreateDefault()

	if base.ID == uuid.Nil {
		t.Fatal("BaseModel.WithCreateDefault should assign a UUID")
	}
	if base.CreatedAt.IsZero() {
		t.Fatal("BaseModel.WithCreateDefault should set CreatedAt")
	}
	if base.UpdatedAt.IsZero() {
		t.Fatal("BaseModel.WithCreateDefault should set UpdatedAt")
	}
}
