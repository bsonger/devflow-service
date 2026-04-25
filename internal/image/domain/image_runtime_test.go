package domain

import (
	"testing"
	"time"
)

func TestBuildImageTargetMainBranchKeepsBaseName(t *testing.T) {
	target, err := BuildImageTarget(ImageRegistryConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
	}, "Portal API", "main", "", time.Date(2026, 4, 8, 13, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildImageTarget returned error: %v", err)
	}
	if target.Name != "portal-api" {
		t.Fatalf("Name = %q want portal-api", target.Name)
	}
	if target.Tag != "20260408-130500" {
		t.Fatalf("Tag = %q want 20260408-130500", target.Tag)
	}
	if target.Ref != "registry.cn-hangzhou.aliyuncs.com/devflow/portal-api:20260408-130500" {
		t.Fatalf("Ref = %q want registry.cn-hangzhou.aliyuncs.com/devflow/portal-api:20260408-130500", target.Ref)
	}
}

func TestBuildImageTargetFeatureBranchAppendsBranch(t *testing.T) {
	target, err := BuildImageTarget(ImageRegistryConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
	}, "Checkout", "feature/new-ui", "", time.Date(2026, 4, 8, 13, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildImageTarget returned error: %v", err)
	}
	if target.Name != "checkout-feature-new-ui" {
		t.Fatalf("Name = %q want checkout-feature-new-ui", target.Name)
	}
}

func TestBuildImageTargetWithExplicitTag(t *testing.T) {
	target, err := BuildImageTarget(ImageRegistryConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
	}, "Portal", "main", "v1.2.3", time.Now())
	if err != nil {
		t.Fatalf("BuildImageTarget returned error: %v", err)
	}
	if target.Tag != "v1.2.3" {
		t.Fatalf("Tag = %q want v1.2.3", target.Tag)
	}
}

func TestBuildImageTargetEmptyNameReturnsError(t *testing.T) {
	_, err := BuildImageTarget(ImageRegistryConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
	}, "", "main", "", time.Now())
	if err == nil {
		t.Fatal("expected error for empty application name")
	}
}

func TestBuildImageTargetEmptyRegistryReturnsError(t *testing.T) {
	_, err := BuildImageTarget(ImageRegistryConfig{
		Registry:  "",
		Namespace: "",
	}, "Portal", "main", "", time.Now())
	if err == nil {
		t.Fatal("expected error for empty registry")
	}
}
