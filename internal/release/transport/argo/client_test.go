package argoclient

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildApplicationInspectionSummarizesSyncTruth(t *testing.T) {
	app := &appv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "meta-service", Namespace: "argocd"},
		Spec: appv1.ApplicationSpec{
			Project: "app",
			Source: &appv1.ApplicationSource{
				RepoURL:        "oci://registry.example.com/devflow/releases/meta-service",
				TargetRevision: "sha256:abc",
			},
			Destination: appv1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "meta-service",
			},
			IgnoreDifferences: appv1.IgnoreDifferences{
				{
					Group: "apps",
					Kind:  "Deployment",
					JSONPointers: []string{
						restartedAtIgnorePointer,
					},
				},
			},
		},
		Status: appv1.ApplicationStatus{
			Sync:   appv1.SyncStatus{Status: appv1.SyncStatusCodeOutOfSync, Revision: "sha256:abc"},
			Health: appv1.AppHealthStatus{Status: "Healthy"},
			OperationState: &appv1.OperationState{
				Phase:   "Succeeded",
				Message: "sync completed with drift remaining",
			},
			Resources: []appv1.ResourceStatus{
				{
					Group:     "apps",
					Kind:      "Deployment",
					Namespace: "meta-service",
					Name:      "meta-service",
					Status:    appv1.SyncStatusCodeOutOfSync,
					Health:    &appv1.HealthStatus{Status: "Healthy"},
				},
				{
					Group:     "",
					Kind:      "Service",
					Namespace: "meta-service",
					Name:      "meta-service",
					Status:    appv1.SyncStatusCodeSynced,
					Health:    &appv1.HealthStatus{Status: "Healthy"},
				},
			},
		},
	}

	inspection := BuildApplicationInspection(app)
	if inspection == nil {
		t.Fatal("expected inspection")
	}
	if inspection.Name != "meta-service" {
		t.Fatalf("name = %q", inspection.Name)
	}
	if inspection.SyncStatus != "OutOfSync" {
		t.Fatalf("sync status = %q", inspection.SyncStatus)
	}
	if inspection.HealthStatus != "Healthy" {
		t.Fatalf("health status = %q", inspection.HealthStatus)
	}
	if inspection.OperationPhase != "Succeeded" {
		t.Fatalf("operation phase = %q", inspection.OperationPhase)
	}
	if inspection.OperationMessage != "sync completed with drift remaining" {
		t.Fatalf("operation message = %q", inspection.OperationMessage)
	}
	if inspection.Revision != "sha256:abc" {
		t.Fatalf("revision = %q", inspection.Revision)
	}
	if inspection.TargetRevision != "sha256:abc" {
		t.Fatalf("target revision = %q", inspection.TargetRevision)
	}
	if inspection.RepoURL != "oci://registry.example.com/devflow/releases/meta-service" {
		t.Fatalf("repoURL = %q", inspection.RepoURL)
	}
	if inspection.DestinationNamespace != "meta-service" {
		t.Fatalf("destination namespace = %q", inspection.DestinationNamespace)
	}
	if inspection.PrimaryWorkloadGroup != "apps" || inspection.PrimaryWorkloadKind != "Deployment" {
		t.Fatalf("primary workload target = %s/%s", inspection.PrimaryWorkloadGroup, inspection.PrimaryWorkloadKind)
	}
	if len(inspection.MetadataCompatibleKinds) != 2 || inspection.MetadataCompatibleKinds[0] != "Deployment" || inspection.MetadataCompatibleKinds[1] != "Rollout" {
		t.Fatalf("metadata compatible kinds = %#v", inspection.MetadataCompatibleKinds)
	}
	if !inspection.RestartedAtIgnoreConfigured {
		t.Fatal("expected restartedAt ignore to be detected")
	}
	if len(inspection.IgnoreDifferenceTargets) != 1 {
		t.Fatalf("ignore targets = %#v", inspection.IgnoreDifferenceTargets)
	}
	if len(inspection.OutOfSyncResources) != 1 {
		t.Fatalf("outOfSync resources = %#v", inspection.OutOfSyncResources)
	}
	if inspection.OutOfSyncResources[0].Kind != "Deployment" || inspection.OutOfSyncResources[0].Name != "meta-service" {
		t.Fatalf("unexpected outOfSync resource = %#v", inspection.OutOfSyncResources[0])
	}
	if len(inspection.RestartedAtCandidates) != 1 {
		t.Fatalf("restartedAt candidates = %#v", inspection.RestartedAtCandidates)
	}
	if inspection.RestartedAtCandidates[0].Kind != "Deployment" {
		t.Fatalf("unexpected restartedAt candidate = %#v", inspection.RestartedAtCandidates[0])
	}
}

func TestBuildApplicationInspectionReportsMissingRestartedAtIgnore(t *testing.T) {
	app := &appv1.Application{
		Spec: appv1.ApplicationSpec{
			IgnoreDifferences: appv1.IgnoreDifferences{{Group: "apps", Kind: "StatefulSet", JSONPointers: []string{"/spec/template/metadata/annotations/example.com~1other"}}},
		},
	}

	inspection := BuildApplicationInspection(app)
	if inspection == nil {
		t.Fatal("expected inspection")
	}
	if inspection.RestartedAtIgnoreConfigured {
		t.Fatalf("expected restartedAt ignore detection to be false: %#v", inspection.IgnoreDifferenceTargets)
	}
	if len(inspection.MetadataCompatibleKinds) != 2 || inspection.MetadataCompatibleKinds[0] != "Deployment" || inspection.MetadataCompatibleKinds[1] != "Rollout" {
		t.Fatalf("metadata compatible kinds = %#v", inspection.MetadataCompatibleKinds)
	}
}

func TestBuildApplicationInspectionRecognizesRolloutPrimaryWorkload(t *testing.T) {
	app := &appv1.Application{
		Spec: appv1.ApplicationSpec{
			IgnoreDifferences: appv1.IgnoreDifferences{{
				Group: "argoproj.io",
				Kind:  "Rollout",
				JSONPointers: []string{restartedAtIgnorePointer},
			}},
		},
		Status: appv1.ApplicationStatus{
			Resources: []appv1.ResourceStatus{{
				Group:  "argoproj.io",
				Kind:   "Rollout",
				Name:   "meta-service",
				Status: appv1.SyncStatusCodeOutOfSync,
			}},
		},
	}

	inspection := BuildApplicationInspection(app)
	if inspection == nil {
		t.Fatal("expected inspection")
	}
	if inspection.PrimaryWorkloadGroup != "argoproj.io" || inspection.PrimaryWorkloadKind != "Rollout" {
		t.Fatalf("primary workload target = %s/%s", inspection.PrimaryWorkloadGroup, inspection.PrimaryWorkloadKind)
	}
	if !inspection.RestartedAtIgnoreConfigured {
		t.Fatal("expected rollout restartedAt ignore to be detected")
	}
	if len(inspection.MetadataCompatibleKinds) != 2 || inspection.MetadataCompatibleKinds[0] != "Deployment" || inspection.MetadataCompatibleKinds[1] != "Rollout" {
		t.Fatalf("metadata compatible kinds = %#v", inspection.MetadataCompatibleKinds)
	}
}
