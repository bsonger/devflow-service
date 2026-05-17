package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/bootstrap"
	runtimeobserver "github.com/bsonger/devflow-service/internal/runtime/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestInitRuntimeStartsBaselineObserversWhenClusterConfigAvailable(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	var kubernetesCfg runtimeobserver.KubernetesRuntimeObserverConfig
	var rolloutCfg runtimeobserver.ReleaseRolloutObserverConfig
	var manifestRuntimeCfg bootstrap.ManifestRuntimeBootstrapConfig
	var releaseRuntimeCfg bootstrap.ReleaseRuntimeBootstrapConfig
	kubernetesCalled := false
	rolloutCalled := false
	manifestRuntimeCalled := false
	releaseRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
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
	startReleaseRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ReleaseRuntimeBootstrapConfig) error {
		releaseRuntimeCalled = true
		releaseRuntimeCfg = cfg
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestRuntimeCalled = true
		manifestRuntimeCfg = cfg
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:           "observer-secret",
			ControlPlaneID:        "cp-1",
			TektonNamespace:       "tekton-observers",
			PollIntervalSeconds:   27,
			ReleaseRuntimeWorkers: 3,
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

	if !kubernetesCalled {
		t.Fatal("expected Kubernetes runtime observer to start")
	}
	if !rolloutCalled {
		t.Fatal("expected release rollout observer to start")
	}
	if releaseRuntimeCalled {
		t.Fatal("expected release runtime reconciler to stay disabled by default")
	}
	if manifestRuntimeCalled {
		t.Fatal("expected manifest runtime reconciler to stay disabled by default")
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
	if releaseRuntimeCfg.Enabled {
		t.Fatal("expected release runtime config to remain zero-valued when disabled")
	}
	if manifestRuntimeCfg.Enabled {
		t.Fatal("expected manifest runtime config to remain zero-valued when disabled")
	}
}

func TestInitRuntimeSkipsObserversWhenClusterConfigUnavailable(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("not in cluster")
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		t.Fatal("kubernetes observer should not start without cluster config")
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		t.Fatal("release rollout observer should not start without cluster config")
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
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

func TestInitRuntimeStartsReleaseRuntimeReconcilerWhenEnabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	enabled := true
	var releaseRuntimeCfg bootstrap.ReleaseRuntimeBootstrapConfig
	releaseRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ReleaseRuntimeBootstrapConfig) error {
		releaseRuntimeCalled = true
		releaseRuntimeCfg = cfg
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:           "observer-secret",
			ControlPlaneID:        "cp-1",
			PollIntervalSeconds:   19,
			ReleaseRuntimeEnabled: &enabled,
			ReleaseRuntimeWorkers: 4,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !releaseRuntimeCalled {
		t.Fatal("expected release runtime reconciler to start")
	}
	if !releaseRuntimeCfg.Enabled {
		t.Fatal("expected release runtime enabled")
	}
	if releaseRuntimeCfg.ControlPlaneID != "cp-1" {
		t.Fatalf("release runtime control plane = %q", releaseRuntimeCfg.ControlPlaneID)
	}
	if releaseRuntimeCfg.PollInterval != 19*time.Second {
		t.Fatalf("release runtime poll interval = %s", releaseRuntimeCfg.PollInterval)
	}
	if releaseRuntimeCfg.Workers != 4 {
		t.Fatalf("release runtime workers = %d", releaseRuntimeCfg.Workers)
	}
	if releaseRuntimeCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("release runtime release base = %q", releaseRuntimeCfg.ReleaseServiceBaseURL)
	}
	if releaseRuntimeCfg.ObserverToken != "observer-secret" {
		t.Fatalf("release runtime observer token = %q", releaseRuntimeCfg.ObserverToken)
	}
}

func TestInitRuntimeStartsReleaseRuntimeReconcilerWithoutPostgres(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	enabled := true
	releaseRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		releaseRuntimeCalled = true
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			ControlPlaneID:        "cp-1",
			ReleaseRuntimeEnabled: &enabled,
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !releaseRuntimeCalled {
		t.Fatal("expected release runtime reconciler to start without postgres")
	}
}

func TestInitRuntimeStartsManifestRuntimeReconcilerWhenEnabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	enabled := true
	var manifestRuntimeCfg bootstrap.ManifestRuntimeBootstrapConfig
	manifestRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestRuntimeCalled = true
		manifestRuntimeCfg = cfg
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:            "observer-secret",
			ControlPlaneID:         "cp-1",
			TektonNamespace:        "tekton-observers",
			TektonPipeline:         "build-pipeline",
			PollIntervalSeconds:    23,
			ManifestRuntimeEnabled: &enabled,
			ManifestRuntimeWorkers: 5,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !manifestRuntimeCalled {
		t.Fatal("expected manifest runtime reconciler to start")
	}
	if !manifestRuntimeCfg.Enabled {
		t.Fatal("expected manifest runtime enabled")
	}
	if manifestRuntimeCfg.ControlPlaneID != "cp-1" {
		t.Fatalf("manifest runtime control plane = %q", manifestRuntimeCfg.ControlPlaneID)
	}
	if manifestRuntimeCfg.PollInterval != 23*time.Second {
		t.Fatalf("manifest runtime poll interval = %s", manifestRuntimeCfg.PollInterval)
	}
	if manifestRuntimeCfg.Workers != 5 {
		t.Fatalf("manifest runtime workers = %d", manifestRuntimeCfg.Workers)
	}
	if manifestRuntimeCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("manifest runtime release base = %q", manifestRuntimeCfg.ReleaseServiceBaseURL)
	}
	if manifestRuntimeCfg.ObserverToken != "observer-secret" {
		t.Fatalf("manifest runtime observer token = %q", manifestRuntimeCfg.ObserverToken)
	}
	if manifestRuntimeCfg.TektonNamespace != "tekton-observers" {
		t.Fatalf("manifest runtime tekton namespace = %q", manifestRuntimeCfg.TektonNamespace)
	}
	if manifestRuntimeCfg.TektonPipeline != "build-pipeline" {
		t.Fatalf("manifest runtime tekton pipeline = %q", manifestRuntimeCfg.TektonPipeline)
	}
}

func TestInitRuntimeSkipsManifestRuntimeReconcilerWhenDisabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	disabled := false
	manifestCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestCalled = true
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			ManifestRuntimeEnabled: &disabled,
			PollIntervalSeconds:    15,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if manifestCalled {
		t.Fatal("expected manifest runtime reconciler to stay disabled")
	}
}

func TestInitRuntimeSkipsLegacyReleaseRolloutObserverWhenReleaseRuntimeEnabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	enabled := true
	legacyCalled := false
	releaseRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		legacyCalled = true
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		releaseRuntimeCalled = true
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
		return nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			ControlPlaneID:        "cp-1",
			ReleaseRuntimeEnabled: &enabled,
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if legacyCalled {
		t.Fatal("expected legacy release rollout observer to stay disabled")
	}
	if !releaseRuntimeCalled {
		t.Fatal("expected release runtime reconciler to start")
	}
}

func installRuntimeConfigTestHooks() func() {
	origCluster := inClusterConfig
	origKubernetes := startKubernetesRuntimeObserverFn
	origRollout := startReleaseRolloutObserverFn
	origReleaseRuntime := startReleaseRuntimeReconcilerFn
	origManifestRuntime := startManifestRuntimeReconcilerFn
	origInitObservability := initObservability

	initObservability = func(context.Context, observabilityOptions) (func(context.Context) error, error) {
		return func(context.Context) error { return nil }, nil
	}

	return func() {
		inClusterConfig = origCluster
		startKubernetesRuntimeObserverFn = origKubernetes
		startReleaseRolloutObserverFn = origRollout
		startReleaseRuntimeReconcilerFn = origReleaseRuntime
		startManifestRuntimeReconcilerFn = origManifestRuntime
		initObservability = origInitObservability
	}
}

var _ = metav1.NamespaceAll
