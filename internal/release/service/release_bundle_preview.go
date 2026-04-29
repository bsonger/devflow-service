package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func newReleaseBundleRecord(bundle *model.ReleaseBundle) *model.ReleaseBundleRecord {
	if bundle == nil {
		return nil
	}
	record := &model.ReleaseBundleRecord{
		ReleaseID:       bundle.ReleaseID,
		Namespace:       strings.TrimSpace(bundle.Namespace),
		ArtifactName:    strings.TrimSpace(bundle.ArtifactName),
		BundleDigest:    strings.TrimSpace(releaseBundleDigest(bundle)),
		RenderedObjects: bundle.RenderedObjects,
		BundleYAML:      releaseBundleCombinedContent(bundle),
	}
	record.WithCreateDefault()
	return record
}

func buildReleaseBundlePreview(release *model.Release, manifest *manifestdomain.Manifest, bundle *model.ReleaseBundleRecord) *model.ReleaseBundlePreview {
	if release == nil || bundle == nil {
		return nil
	}
	return &model.ReleaseBundlePreview{
		ReleaseID:      release.ID,
		ManifestID:     release.ManifestID,
		ApplicationID:  release.ApplicationID,
		EnvironmentID:  release.EnvironmentID,
		Strategy:       release.Strategy,
		Namespace:      bundle.Namespace,
		ArtifactName:   bundle.ArtifactName,
		BundleDigest:   bundle.BundleDigest,
		Artifact:       buildReleaseBundleArtifact(release),
		FrozenInputs:   buildFrozenInputs(release, manifest),
		RenderedBundle: buildRenderedBundleView(bundle),
		RenderedAt:     bundle.CreatedAt,
		PublishedAt:    derivePublishedAt(release.Steps),
	}
}

func buildReleaseBundleSummary(release *model.Release, bundle *model.ReleaseBundleRecord) *model.ReleaseBundleSummary {
	if bundle == nil {
		return nil
	}
	counts := summarizeReleaseBundleResources(bundle.RenderedObjects)
	return &model.ReleaseBundleSummary{
		Available:           true,
		Namespace:           bundle.Namespace,
		ArtifactName:        bundle.ArtifactName,
		BundleDigest:        bundle.BundleDigest,
		PrimaryWorkloadKind: primaryWorkloadKind(bundle.RenderedObjects),
		ResourceCounts:      counts,
		Artifact:            buildReleaseBundleArtifact(release),
		RenderedAt:          timePtr(bundle.CreatedAt),
		PublishedAt:         derivePublishedAt(release.Steps),
	}
}

func buildReleaseBundleArtifact(release *model.Release) *model.ReleaseBundleArtifact {
	if release == nil {
		return nil
	}
	artifact := &model.ReleaseBundleArtifact{
		Repository: strings.TrimSpace(release.ArtifactRepository),
		Tag:        strings.TrimSpace(release.ArtifactTag),
		Digest:     strings.TrimSpace(release.ArtifactDigest),
		Ref:        strings.TrimSpace(release.ArtifactRef),
	}
	if artifact.Repository == "" && artifact.Tag == "" && artifact.Digest == "" && artifact.Ref == "" {
		return nil
	}
	return artifact
}

func buildFrozenInputs(release *model.Release, manifest *manifestdomain.Manifest) model.ReleaseBundleFrozenInputs {
	out := model.ReleaseBundleFrozenInputs{}
	if manifest != nil {
		out.ManifestSummary = model.ReleaseBundleManifestSummary{
			ManifestID:  manifest.ID,
			CommitHash:  strings.TrimSpace(manifest.CommitHash),
			ImageRef:    strings.TrimSpace(manifest.ImageRef),
			ImageDigest: strings.TrimSpace(manifest.ImageDigest),
		}
		out.Services = make([]model.ReleaseFrozenService, 0, len(manifest.ServicesSnapshot))
		for _, item := range manifest.ServicesSnapshot {
			service := model.ReleaseFrozenService{
				Name:  strings.TrimSpace(item.Name),
				Ports: make([]model.ReleaseFrozenServicePort, 0, len(item.Ports)),
			}
			for _, port := range item.Ports {
				service.Ports = append(service.Ports, model.ReleaseFrozenServicePort{
					Name:        strings.TrimSpace(port.Name),
					ServicePort: port.ServicePort,
					TargetPort:  port.TargetPort,
					Protocol:    strings.TrimSpace(port.Protocol),
				})
			}
			out.Services = append(out.Services, service)
		}
		out.Workload = model.ReleaseFrozenWorkload{
			Replicas:           manifest.WorkloadConfigSnapshot.Replicas,
			ServiceAccountName: strings.TrimSpace(manifest.WorkloadConfigSnapshot.ServiceAccountName),
			Resources:          manifest.WorkloadConfigSnapshot.Resources,
			Probes:             manifest.WorkloadConfigSnapshot.Probes,
			Env:                manifest.WorkloadConfigSnapshot.Env,
			Labels:             manifest.WorkloadConfigSnapshot.Labels,
			Annotations:        manifest.WorkloadConfigSnapshot.Annotations,
		}
	}
	if release != nil {
		out.AppConfig = release.AppConfigSnapshot
		out.Routes = release.RoutesSnapshot
	}
	return out
}

func buildRenderedBundleView(bundle *model.ReleaseBundleRecord) model.ReleaseRenderedBundleView {
	if bundle == nil {
		return model.ReleaseRenderedBundleView{}
	}
	return model.ReleaseRenderedBundleView{
		ResourceGroups:    groupRenderedObjects(bundle.RenderedObjects),
		RenderedResources: buildRenderedResourceViews(bundle.RenderedObjects),
		Files:             buildReleaseBundleFileViews(bundle.RenderedObjects, bundle.BundleYAML),
	}
}

func groupRenderedObjects(items []model.ReleaseRenderedResource) []model.ReleaseResourceGroup {
	out := make([]model.ReleaseResourceGroup, 0)
	indexByKind := make(map[string]int, len(items))
	for _, item := range items {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			continue
		}
		index, ok := indexByKind[kind]
		if !ok {
			index = len(out)
			indexByKind[kind] = index
			out = append(out, model.ReleaseResourceGroup{Kind: kind})
		}
		out[index].Items = append(out[index].Items, model.ReleaseRenderedResourceRef{
			Name:      strings.TrimSpace(item.Name),
			Namespace: strings.TrimSpace(item.Namespace),
		})
	}
	return out
}

func buildRenderedResourceViews(items []model.ReleaseRenderedResource) []model.ReleaseRenderedResourceView {
	out := make([]model.ReleaseRenderedResourceView, 0, len(items))
	for _, item := range items {
		summary := summarizeRenderedObject(item)
		view := model.ReleaseRenderedResourceView{
			Kind:      strings.TrimSpace(item.Kind),
			Name:      strings.TrimSpace(item.Name),
			Namespace: strings.TrimSpace(item.Namespace),
			YAML:      item.YAML,
		}
		if len(summary) > 0 {
			view.Summary = summary
		}
		out = append(out, view)
	}
	return out
}

func buildReleaseBundleFileViews(items []model.ReleaseRenderedResource, bundleYAML string) []model.ReleaseBundleFileView {
	files := make([]model.ReleaseBundleFileView, 0, len(items)+1)
	for i, item := range items {
		files = append(files, model.ReleaseBundleFileView{
			Path:    fmt.Sprintf("%02d-%s-%s.yaml", i+1, strings.ToLower(strings.TrimSpace(item.Kind)), item.Name),
			Content: item.YAML,
		})
	}
	if strings.TrimSpace(bundleYAML) != "" {
		files = append(files, model.ReleaseBundleFileView{
			Path:    "bundle.yaml",
			Content: bundleYAML,
		})
	}
	return files
}

func buildReleaseBundleFiles(items []model.ReleaseRenderedResource, bundleYAML string) []model.ReleaseBundleFile {
	files := make([]model.ReleaseBundleFile, 0, len(items)+1)
	for i, item := range items {
		files = append(files, model.ReleaseBundleFile{
			Path:    fmt.Sprintf("%02d-%s-%s.yaml", i+1, strings.ToLower(strings.TrimSpace(item.Kind)), item.Name),
			Content: item.YAML,
		})
	}
	if strings.TrimSpace(bundleYAML) != "" {
		files = append(files, model.ReleaseBundleFile{
			Path:    "bundle.yaml",
			Content: bundleYAML,
		})
	}
	return files
}

func buildReleaseBundleFromRecord(release *model.Release, bundle *model.ReleaseBundleRecord) *model.ReleaseBundle {
	if release == nil || bundle == nil {
		return nil
	}
	return &model.ReleaseBundle{
		ReleaseID:       release.ID,
		ApplicationID:   release.ApplicationID,
		EnvironmentID:   release.EnvironmentID,
		Namespace:       bundle.Namespace,
		ArtifactName:    bundle.ArtifactName,
		RenderedObjects: bundle.RenderedObjects,
		Files:           buildReleaseBundleFiles(bundle.RenderedObjects, bundle.BundleYAML),
	}
}

func derivePublishedAt(steps []model.ReleaseStep) *time.Time {
	for _, step := range steps {
		if step.Code != "publish_bundle" {
			continue
		}
		if step.Status != model.StepSucceeded || step.EndTime == nil {
			return nil
		}
		value := *step.EndTime
		return &value
	}
	return nil
}

func summarizeReleaseBundleResources(items []model.ReleaseRenderedResource) model.ReleaseBundleResourceCounts {
	counts := model.ReleaseBundleResourceCounts{}
	for _, item := range items {
		switch strings.TrimSpace(item.Kind) {
		case "ConfigMap":
			counts.ConfigMaps++
		case "Service":
			counts.Services++
		case "Deployment":
			counts.Deployments++
		case "Rollout":
			counts.Rollouts++
		case "VirtualService":
			counts.VirtualServices++
		}
	}
	counts.Total = len(items)
	return counts
}

func primaryWorkloadKind(items []model.ReleaseRenderedResource) string {
	for _, item := range items {
		switch strings.TrimSpace(item.Kind) {
		case "Rollout":
			return "Rollout"
		case "Deployment":
			return "Deployment"
		}
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	v := value
	return &v
}

func summarizeRenderedObject(item model.ReleaseRenderedResource) map[string]any {
	object := item.Object
	if len(object) == 0 {
		return nil
	}
	switch strings.TrimSpace(item.Kind) {
	case "ConfigMap":
		return summarizeConfigMap(object)
	case "Service":
		return summarizeService(object)
	case "Deployment":
		return summarizeDeploymentLike(object, false)
	case "Rollout":
		return summarizeDeploymentLike(object, true)
	case "VirtualService":
		return summarizeVirtualService(object)
	default:
		return nil
	}
}

func summarizeConfigMap(object map[string]any) map[string]any {
	data := mapValue(object["data"])
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return map[string]any{
		"data_keys":  keys,
		"item_count": len(keys),
	}
}

func summarizeService(object map[string]any) map[string]any {
	spec := mapValue(object["spec"])
	if len(spec) == 0 {
		return nil
	}
	summary := map[string]any{}
	ports := make([]map[string]any, 0)
	for _, raw := range sliceValue(spec["ports"]) {
		portMap := mapValue(raw)
		if len(portMap) == 0 {
			continue
		}
		port := map[string]any{}
		if value := stringValue(portMap["name"]); value != "" {
			port["name"] = value
		}
		if value, ok := intValue(portMap["port"]); ok {
			port["port"] = value
		}
		if value, ok := intValue(portMap["targetPort"]); ok {
			port["target_port"] = value
		}
		if value := stringValue(portMap["protocol"]); value != "" {
			port["protocol"] = value
		}
		if len(port) > 0 {
			ports = append(ports, port)
		}
	}
	if len(ports) > 0 {
		summary["ports"] = ports
	}
	if selector := mapValue(spec["selector"]); len(selector) > 0 {
		summary["selector"] = selector
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func summarizeDeploymentLike(object map[string]any, isRollout bool) map[string]any {
	spec := mapValue(object["spec"])
	if len(spec) == 0 {
		return nil
	}
	summary := map[string]any{}
	if value, ok := intValue(spec["replicas"]); ok {
		summary["replicas"] = value
	}
	templateSpec := mapValue(mapValue(spec["template"])["spec"])
	if len(templateSpec) > 0 {
		if value := stringValue(templateSpec["serviceAccountName"]); value != "" {
			summary["service_account_name"] = value
		}
		containers := sliceValue(templateSpec["containers"])
		if len(containers) > 0 {
			container := mapValue(containers[0])
			if value := stringValue(container["image"]); value != "" {
				summary["image"] = value
			}
			if resources := mapValue(container["resources"]); len(resources) > 0 {
				summary["resources"] = resources
			}
			envNames := make([]string, 0)
			for _, raw := range sliceValue(container["env"]) {
				if name := stringValue(mapValue(raw)["name"]); name != "" {
					envNames = append(envNames, name)
				}
			}
			if len(envNames) > 0 {
				summary["env_names"] = envNames
			}
			probeTypes := make([]string, 0, 3)
			for _, key := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
				if len(mapValue(container[key])) > 0 {
					probeTypes = append(probeTypes, key)
				}
			}
			if len(probeTypes) > 0 {
				summary["probe_types"] = probeTypes
			}
			for _, raw := range sliceValue(container["volumeMounts"]) {
				mount := mapValue(raw)
				if stringValue(mount["name"]) != "app-config" {
					continue
				}
				if value := stringValue(mount["mountPath"]); value != "" {
					summary["config_mount_path"] = value
					break
				}
			}
		}
	}
	if isRollout {
		strategy := mapValue(spec["strategy"])
		if blueGreen := mapValue(strategy["blueGreen"]); len(blueGreen) > 0 {
			summary["strategy_type"] = "blueGreen"
			details := map[string]any{}
			if value := stringValue(blueGreen["activeService"]); value != "" {
				details["active_service"] = value
			}
			if value := stringValue(blueGreen["previewService"]); value != "" {
				details["preview_service"] = value
			}
			if len(details) > 0 {
				summary["blue_green"] = details
			}
		}
		if canary := mapValue(strategy["canary"]); len(canary) > 0 {
			summary["strategy_type"] = "canary"
			details := map[string]any{}
			if value := stringValue(canary["stableService"]); value != "" {
				details["stable_service"] = value
			}
			if value := stringValue(canary["canaryService"]); value != "" {
				details["canary_service"] = value
			}
			steps := make([]int, 0)
			for _, raw := range sliceValue(canary["steps"]) {
				if value, ok := intValue(mapValue(raw)["setWeight"]); ok {
					steps = append(steps, value)
				}
			}
			if len(steps) > 0 {
				details["steps"] = steps
			}
			if len(details) > 0 {
				summary["canary"] = details
			}
		}
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func summarizeVirtualService(object map[string]any) map[string]any {
	spec := mapValue(object["spec"])
	if len(spec) == 0 {
		return nil
	}
	summary := map[string]any{}
	hosts := stringSlice(sliceValue(spec["hosts"]))
	if len(hosts) > 0 {
		summary["hosts"] = hosts
	}
	routes := make([]map[string]any, 0)
	for _, raw := range sliceValue(spec["http"]) {
		httpRoute := mapValue(raw)
		if len(httpRoute) == 0 {
			continue
		}
		entry := map[string]any{}
		matches := sliceValue(httpRoute["match"])
		if len(matches) > 0 {
			if prefix := stringValue(mapValue(mapValue(matches[0])["uri"])["prefix"]); prefix != "" {
				entry["path_prefix"] = prefix
			}
		}
		destinations := make([]map[string]any, 0)
		for _, routeRaw := range sliceValue(httpRoute["route"]) {
			routeMap := mapValue(routeRaw)
			destination := mapValue(routeMap["destination"])
			if len(destination) == 0 {
				continue
			}
			dest := map[string]any{}
			if value := stringValue(destination["host"]); value != "" {
				dest["host"] = value
			}
			if value, ok := intValue(mapValue(destination["port"])["number"]); ok {
				dest["port"] = value
			}
			if value, ok := intValue(routeMap["weight"]); ok {
				dest["weight"] = value
			}
			if len(dest) > 0 {
				destinations = append(destinations, dest)
			}
		}
		if len(destinations) > 0 {
			entry["destinations"] = destinations
		}
		if len(entry) > 0 {
			routes = append(routes, entry)
		}
	}
	if len(routes) > 0 {
		summary["routes"] = routes
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func mapValue(value any) map[string]any {
	out, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return out
}

func sliceValue(value any) []any {
	out, ok := value.([]any)
	if !ok {
		return nil
	}
	return out
}

func stringSlice(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if parsed := stringValue(value); parsed != "" {
			out = append(out, parsed)
		}
	}
	return out
}

func stringValue(value any) string {
	switch current := value.(type) {
	case string:
		return strings.TrimSpace(current)
	case fmt.Stringer:
		return strings.TrimSpace(current.String())
	default:
		return ""
	}
}

func intValue(value any) (int, bool) {
	switch current := value.(type) {
	case int:
		return current, true
	case int32:
		return int(current), true
	case int64:
		return int(current), true
	case float64:
		return int(current), true
	case float32:
		return int(current), true
	case jsonNumber:
		parsed, err := strconv.Atoi(string(current))
		return parsed, err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(current))
		return parsed, err == nil
	default:
		return 0, false
	}
}

type jsonNumber string
