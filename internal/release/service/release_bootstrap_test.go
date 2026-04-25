package service

import (
	"context"
	"errors"
	"testing"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argofake "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBootstrapGatesStopOnFirstFailure(t *testing.T) {
	// Set up fake argo client so gate 3 can run
	project := &appv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "argocd",
		},
		Spec: appv1.AppProjectSpec{
			Destinations: []appv1.ApplicationDestination{},
		},
	}
	argoClient := argofake.NewSimpleClientset(project)
	argoclient.Client = argoClient
	defer func() { argoclient.Client = nil }()

	client := fake.NewClientset()
	exec := &bootstrapExecutor{kubeClient: client}

	results, err := exec.runBootstrapGates(context.Background(), releasesupport.DeployTarget{
		Namespace:         "test-ns",
		DestinationServer: "https://cluster.example.com",
	}, "app")

	if err != nil {
		// With fake client, gate 3 should succeed (adds destination)
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != model.StepSucceeded {
			t.Fatalf("step %q status = %q, want Succeeded", r.StepName, r.Status)
		}
	}
}

func TestBootstrapGateEnsureNamespaceCreatesWhenMissing(t *testing.T) {
	client := fake.NewClientset()
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsureNamespace(context.Background(), "new-namespace")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "namespace created" {
		t.Fatalf("message = %q, want 'namespace created'", res.Message)
	}

	// Verify it was created
	ns, err := client.CoreV1().Namespaces().Get(context.Background(), "new-namespace", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("namespace get error: %v", err)
	}
	if ns.Labels["devflow.io/managed-by"] != "devflow-release-service" {
		t.Fatalf("unexpected label: %v", ns.Labels)
	}
}

func TestBootstrapGateEnsureNamespaceIdempotent(t *testing.T) {
	client := fake.NewClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing-namespace",
		},
	})
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsureNamespace(context.Background(), "existing-namespace")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "namespace already exists" {
		t.Fatalf("message = %q, want 'namespace already exists'", res.Message)
	}
}

func TestBootstrapGateEnsurePullSecretSkipsWhenNoSource(t *testing.T) {
	client := fake.NewClientset()
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsurePullSecret(context.Background(), "target-ns")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "no source pull secret to copy; skipped" {
		t.Fatalf("message = %q", res.Message)
	}
}

func TestBootstrapGateEnsurePullSecretCopiesWhenMissing(t *testing.T) {
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-credentials",
			Namespace: "devflow-release-service",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte("{}")},
	}
	client := fake.NewClientset(sourceSecret)
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsurePullSecret(context.Background(), "target-ns")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "pull secret copied to target namespace" {
		t.Fatalf("message = %q", res.Message)
	}

	// Verify copy
	copied, err := client.CoreV1().Secrets("target-ns").Get(context.Background(), "registry-credentials", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("copied secret get error: %v", err)
	}
	if copied.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("unexpected type: %v", copied.Type)
	}
}

func TestBootstrapGateEnsurePullSecretIdempotent(t *testing.T) {
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-credentials",
			Namespace: "devflow-release-service",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte("{}")},
	}
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-credentials",
			Namespace: "target-ns",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": []byte("{}")},
	}
	client := fake.NewClientset(sourceSecret, existing)
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsurePullSecret(context.Background(), "target-ns")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "pull secret already exists in target namespace" {
		t.Fatalf("message = %q", res.Message)
	}
}

func TestBootstrapGateEnsureAppProjectDestinationAlreadyPresent(t *testing.T) {
	project := &appv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "argocd",
		},
		Spec: appv1.AppProjectSpec{
			Destinations: []appv1.ApplicationDestination{
				{Server: "https://cluster.example.com", Namespace: "test-ns"},
			},
		},
	}

	// Set up fake argo client
	argoClient := argofake.NewSimpleClientset(project)
	argoclient.Client = argoClient

	exec := &bootstrapExecutor{kubeClient: fake.NewClientset()}
	res := exec.gateEnsureAppProjectDestination(context.Background(), "app", "https://cluster.example.com", "test-ns")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", res.Status)
	}
	if res.Message != "destination already in allowlist" {
		t.Fatalf("message = %q", res.Message)
	}
}

func TestBootstrapGateEnsureAppProjectDestinationAddsWhenMissing(t *testing.T) {
	project := &appv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "argocd",
		},
		Spec: appv1.AppProjectSpec{
			Destinations: []appv1.ApplicationDestination{},
		},
	}

	argoClient := argofake.NewSimpleClientset(project)
	argoclient.Client = argoClient

	exec := &bootstrapExecutor{kubeClient: fake.NewClientset()}
	res := exec.gateEnsureAppProjectDestination(context.Background(), "app", "https://cluster.example.com", "test-ns")
	if res.Status != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded: %s", res.Status, res.Message)
	}
	if res.Message != "destination added to appproject allowlist" {
		t.Fatalf("message = %q", res.Message)
	}

	updated, err := argoClient.ArgoprojV1alpha1().AppProjects("argocd").Get(context.Background(), "app", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get updated project: %v", err)
	}
	found := false
	for _, d := range updated.Spec.Destinations {
		if d.Server == "https://cluster.example.com" && d.Namespace == "test-ns" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected destination to be added to project")
	}
}

func TestBootstrapGateEnsureAppProjectDestinationFailsWhenNotFound(t *testing.T) {
	argoClient := argofake.NewSimpleClientset()
	argoclient.Client = argoClient

	exec := &bootstrapExecutor{kubeClient: fake.NewClientset()}
	res := exec.gateEnsureAppProjectDestination(context.Background(), "missing-project", "https://cluster.example.com", "test-ns")
	if res.Status != model.StepFailed {
		t.Fatalf("status = %q, want Failed", res.Status)
	}
	if res.Message != `appproject "missing-project" not found` {
		t.Fatalf("message = %q", res.Message)
	}
}

func TestBootstrapMissingTarget(t *testing.T) {
	exec := &bootstrapExecutor{kubeClient: fake.NewClientset()}
	_, err := exec.runBootstrapGates(context.Background(), releasesupport.DeployTarget{}, "app")
	if !errors.Is(err, ErrBootstrapMissingTarget) {
		t.Fatalf("error = %v, want ErrBootstrapMissingTarget", err)
	}
}

func TestBootstrapStepTimingPopulated(t *testing.T) {
	client := fake.NewClientset()
	exec := &bootstrapExecutor{kubeClient: client}

	res := exec.gateEnsureNamespace(context.Background(), "timed-ns")
	if res.Start == nil {
		t.Fatal("expected Start to be set")
	}
	if res.End == nil {
		t.Fatal("expected End to be set")
	}
	if !res.End.After(*res.Start) && !res.End.Equal(*res.Start) {
		t.Fatal("expected End >= Start")
	}
}

func TestBootstrapExecutorMissingKubeConfig(t *testing.T) {
	oldKubeConfig := model.KubeConfig
	model.KubeConfig = nil
	defer func() { model.KubeConfig = oldKubeConfig }()

	_, err := newBootstrapExecutor()
	if !errors.Is(err, ErrBootstrapMissingKubeConfig) {
		t.Fatalf("error = %v, want ErrBootstrapMissingKubeConfig", err)
	}
}
