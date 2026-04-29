package config

import (
	"context"
	"errors"
	"testing"
	"time"

	runtimeobserver "github.com/bsonger/devflow-service/internal/runtime/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestInitRuntimeStartsAllObserversWhenClusterConfigAvailable(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	var tektonCfg runtimeobserver.TektonManifestObserverConfig
	var kubernetesCfg runtimeobserver.KubernetesRuntimeObserverConfig
	var rolloutCfg runtimeobserver.ReleaseRolloutObserverConfig
	tektonCalled := false
	kubernetesCalled := false
	rolloutCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startTektonManifestObserverFn = func(_ context.Context, cfg *rest.Config, observerCfg runtimeobserver.TektonManifestObserverConfig) error {
		tektonCalled = true
		if cfg == nil {
			t.Fatal("tekton observer received nil rest config")
		}
		tektonCfg = observerCfg
		return nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, cfg *rest.Config, observerCfg runtimeobserver.KubernetesRuntimeObserverConfig) error {
		kubernetesCalled = true
		if cfg == nil {
			t.Fatal("kubernetes observer received nil rest config")
		}
		kubernetesCfg = observerCfg
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, cfg *rest.Config, observerCfg runtimeobserver.ReleaseRolloutObserverConfig) error {
		rolloutCalled = true
		if cfg == nil {
			t.Fatal("release rollout observer received nil rest config")
		}
		rolloutCfg = observerCfg
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:         "observer-secret",
			TektonNamespace:     "tekton-observers",
			PollIntervalSeconds: 27,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitRuntime returned nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !tektonCalled {
		t.Fatal("expected Tekton manifest observer to start")
	}
	if !kubernetesCalled {
		t.Fatal("expected Kubernetes runtime observer to start")
	}
	if !rolloutCalled {
		t.Fatal("expected release rollout observer to start")
	}

	if tektonCfg.TektonNamespace != "tekton-observers" {
		t.Fatalf("tekton namespace = %q", tektonCfg.TektonNamespace)
	}
	if tektonCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("tekton release base = %q", tektonCfg.ReleaseServiceBaseURL)
	}
	if tektonCfg.ObserverToken != "observer-secret" {
		t.Fatalf("tekton observer token = %q", tektonCfg.ObserverToken)
	}
	if tektonCfg.PollInterval != 27*time.Second {
		t.Fatalf("tekton poll interval = %s", tektonCfg.PollInterval)
	}

	if !kubernetesCfg.Enabled {
		t.Fatal("expected kubernetes observer enabled")
	}
	if kubernetesCfg.PollInterval != 27*time.Second {
		t.Fatalf("kubernetes poll interval = %s", kubernetesCfg.PollInterval)
	}

	if !rolloutCfg.Enabled {
		t.Fatal("expected rollout observer enabled")
	}
	if rolloutCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("rollout release base = %q", rolloutCfg.ReleaseServiceBaseURL)
	}
	if rolloutCfg.ObserverToken != "observer-secret" {
		t.Fatalf("rollout observer token = %q", rolloutCfg.ObserverToken)
	}
	if rolloutCfg.PollInterval != 27*time.Second {
		t.Fatalf("rollout poll interval = %s", rolloutCfg.PollInterval)
	}
}

func TestInitRuntimeSkipsObserversWhenClusterConfigUnavailable(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("not in cluster")
	}
	startTektonManifestObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.TektonManifestObserverConfig) error {
		t.Fatal("tekton observer should not start without cluster config")
		return nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		t.Fatal("kubernetes observer should not start without cluster config")
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		t.Fatal("release rollout observer should not start without cluster config")
		return nil
	}

	shutdown, err := InitRuntime(context.Background(), &Config{}, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitRuntime returned nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func installRuntimeConfigTestHooks() func() {
	origCluster := inClusterConfig
	origTekton := startTektonManifestObserverFn
	origKubernetes := startKubernetesRuntimeObserverFn
	origRollout := startReleaseRolloutObserverFn
	origInitObservability := initObservability

	initObservability = func(context.Context, observabilityOptions) (func(context.Context) error, error) {
		return func(context.Context) error { return nil }, nil
	}

	return func() {
		inClusterConfig = origCluster
		startTektonManifestObserverFn = origTekton
		startKubernetesRuntimeObserverFn = origKubernetes
		startReleaseRolloutObserverFn = origRollout
		initObservability = origInitObservability
	}
}

var _ = metav1.NamespaceAll
