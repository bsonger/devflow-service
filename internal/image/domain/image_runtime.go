package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var imageSegmentSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)
var imageDashCollapser = regexp.MustCompile(`-+`)

type ImageRegistryConfig struct {
	Registry  string
	Namespace string
	Username  string
	Password  string
}

type ImageTarget struct {
	Name string
	Tag  string
	Ref  string
}

var imageTagSanitizer = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func (c ImageRegistryConfig) Repository() string {
	registry := strings.TrimSuffix(strings.TrimSpace(c.Registry), "/")
	namespace := strings.Trim(strings.TrimSpace(c.Namespace), "/")
	if namespace == "" {
		return registry
	}
	return registry + "/" + namespace
}

func BuildImageTarget(cfg ImageRegistryConfig, applicationName, branch, tag string, now time.Time) (ImageTarget, error) {
	baseName := normalizeImageSegment(applicationName)
	if baseName == "" {
		return ImageTarget{}, fmt.Errorf("application name produced empty image name")
	}

	normalizedBranch := normalizeImageSegment(branch)
	if normalizedBranch == "" {
		normalizedBranch = "main"
	}

	imageName := baseName
	if normalizedBranch != "main" {
		imageName = baseName + "-" + normalizedBranch
	}

	repository := cfg.Repository()
	if repository == "" {
		return ImageTarget{}, fmt.Errorf("image registry repository is empty")
	}

	tag = normalizeImageTag(tag)
	if tag == "" {
		tag = now.UTC().Format("20060102-150405")
	}
	return ImageTarget{
		Name: imageName,
		Tag:  tag,
		Ref:  repository + "/" + imageName + ":" + tag,
	}, nil
}

func normalizeImageSegment(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = imageSegmentSanitizer.ReplaceAllString(trimmed, "-")
	trimmed = imageDashCollapser.ReplaceAllString(trimmed, "-")
	return strings.Trim(trimmed, "-")
}

func normalizeImageTag(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = imageTagSanitizer.ReplaceAllString(trimmed, "-")
	trimmed = strings.Trim(trimmed, ".-")
	return trimmed
}
