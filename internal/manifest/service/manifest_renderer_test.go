package service

import "testing"

func TestResolveWorkloadImageRefPrefersDigest(t *testing.T) {
	got, anns, err := resolveWorkloadImageRef("registry.cn-hangzhou.aliyuncs.com/devflow/demo", "20260411-120000", "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "registry.cn-hangzhou.aliyuncs.com/devflow/demo@sha256:abc" {
		t.Fatalf("got %q", got)
	}
	if anns["devflow.io/image-tag"] != "20260411-120000" {
		t.Fatalf("missing tag annotation")
	}
	if anns["devflow.io/image-ref"] != "registry.cn-hangzhou.aliyuncs.com/devflow/demo:20260411-120000" {
		t.Fatalf("missing tag ref annotation")
	}
}

func TestResolveWorkloadImageRefFallsBackToTag(t *testing.T) {
	got, anns, err := resolveWorkloadImageRef("registry.cn-hangzhou.aliyuncs.com/devflow/demo", "20260411-120000", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "registry.cn-hangzhou.aliyuncs.com/devflow/demo:20260411-120000" {
		t.Fatalf("got %q", got)
	}
	if len(anns) != 0 {
		t.Fatalf("unexpected annotations %+v", anns)
	}
}

func TestResolveWorkloadImageRefRejectsUndeployableImage(t *testing.T) {
	if _, _, err := resolveWorkloadImageRef("registry.cn-hangzhou.aliyuncs.com/devflow/demo", "", ""); err != ErrManifestImageNotDeployable {
		t.Fatalf("got err %v want %v", err, ErrManifestImageNotDeployable)
	}
}
