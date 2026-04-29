package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"sigs.k8s.io/yaml"
)

func buildReleaseBundle(namespace, applicationName string, manifest *manifestdomain.Manifest, release *model.Release) (*model.ReleaseBundle, error) {
	if manifest == nil {
		return nil, sharederrs.Required("manifest")
	}
	if release == nil {
		return nil, sharederrs.Required("release")
	}
	if strings.TrimSpace(applicationName) == "" {
		applicationName = deriveReleaseApplicationName(manifest)
	}

	rendered, err := renderReleaseBundleResources(namespace, applicationName, manifest, release)
	if err != nil {
		return nil, err
	}
	bundle := &model.ReleaseBundle{
		ReleaseID:       release.ID,
		ApplicationID:   release.ApplicationID,
		EnvironmentID:   release.EnvironmentID,
		Namespace:       namespace,
		ArtifactName:    applicationName,
		Resources:       model.ReleaseBundleResources{Services: []model.ReleaseRenderedResource{}},
		RenderedObjects: make([]model.ReleaseRenderedResource, 0, len(rendered)),
		Files:           []model.ReleaseBundleFile{},
	}
	for _, item := range rendered {
		bundle.RenderedObjects = append(bundle.RenderedObjects, item)
		switch strings.ToLower(strings.TrimSpace(item.Kind)) {
		case "configmap":
			bundle.Resources.ConfigMap = &item
		case "deployment":
			bundle.Resources.Deployment = &item
		case "rollout":
			bundle.Resources.Rollout = &item
		case "service":
			bundle.Resources.Services = append(bundle.Resources.Services, item)
		case "virtualservice":
			bundle.Resources.VirtualService = &item
		}
	}

	combined := make([]string, 0, len(bundle.RenderedObjects))
	for _, item := range bundle.RenderedObjects {
		combined = append(combined, strings.TrimSpace(item.YAML))
		bundle.Files = append(bundle.Files, model.ReleaseBundleFile{
			Path:    fmt.Sprintf("%02d-%s-%s.yaml", len(bundle.Files)+1, strings.ToLower(item.Kind), item.Name),
			Content: item.YAML,
		})
	}
	if len(combined) > 0 {
		bundle.Files = append(bundle.Files, model.ReleaseBundleFile{
			Path:    "bundle.yaml",
			Content: strings.Join(combined, "\n---\n") + "\n",
		})
	}
	return bundle, nil
}

func renderReleaseBundleResources(namespace, applicationName string, manifest *manifestdomain.Manifest, release *model.Release) ([]model.ReleaseRenderedResource, error) {
	objects := make([]model.ReleaseRenderedResource, 0, len(manifest.ServicesSnapshot)+4)
	if configMap := buildReleaseConfigMap(namespace, applicationName, release); configMap != nil {
		item, err := marshalReleaseRenderedObject("ConfigMap", applicationName, namespace, configMap)
		if err != nil {
			return nil, err
		}
		objects = append(objects, item)
	}
	if serviceAccount := buildReleaseServiceAccount(namespace, manifest.WorkloadConfigSnapshot.ServiceAccountName); serviceAccount != nil {
		item, err := marshalReleaseRenderedObject("ServiceAccount", strings.TrimSpace(manifest.WorkloadConfigSnapshot.ServiceAccountName), namespace, serviceAccount)
		if err != nil {
			return nil, err
		}
		objects = append(objects, item)
	}

	serviceResources, err := buildReleaseServiceResources(namespace, manifest.ServicesSnapshot, release)
	if err != nil {
		return nil, err
	}
	objects = append(objects, serviceResources...)

	workloadResource, err := buildReleaseWorkloadResource(namespace, applicationName, manifest, release)
	if err != nil {
		return nil, err
	}
	objects = append(objects, workloadResource)

	if virtualService := buildReleaseVirtualService(namespace, applicationName, release.RoutesSnapshot); virtualService != nil {
		item, err := marshalReleaseRenderedObject("VirtualService", applicationName, namespace, virtualService)
		if err != nil {
			return nil, err
		}
		objects = append(objects, item)
	}
	return objects, nil
}

func buildReleaseServiceAccount(namespace, name string) map[string]any {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	metadata := map[string]any{"name": name}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata":   metadata,
	}
}

func buildReleaseConfigMap(namespace, applicationName string, release *model.Release) map[string]any {
	if release == nil {
		return nil
	}
	data := map[string]string{}
	for key, value := range release.AppConfigSnapshot.Data {
		data[key] = value
	}
	if len(data) == 0 {
		for _, file := range release.AppConfigSnapshot.Files {
			if strings.TrimSpace(file.Name) == "" {
				continue
			}
			data[file.Name] = file.Content
		}
	}
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(keys))
	for _, key := range keys {
		ordered[key] = data[key]
	}
	metadata := map[string]any{"name": applicationName}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   metadata,
		"data":       ordered,
	}
}

func buildReleaseServiceResources(namespace string, services []manifestdomain.ManifestService, release *model.Release) ([]model.ReleaseRenderedResource, error) {
	extras := 0
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen, model.Canary:
		extras = 1
	}
	out := make([]model.ReleaseRenderedResource, 0, len(services)+extras)
	for i, service := range services {
		ports := make([]map[string]any, 0, len(service.Ports))
		for _, port := range service.Ports {
			ports = append(ports, map[string]any{
				"name":       port.Name,
				"port":       port.ServicePort,
				"targetPort": port.TargetPort,
				"protocol":   releaseDefaultProtocol(port.Protocol),
			})
		}
		metadata := map[string]any{"name": service.Name}
		if namespace != "" {
			metadata["namespace"] = namespace
		}
		obj := map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   metadata,
			"spec": map[string]any{
				"selector": map[string]any{"app.kubernetes.io/name": service.Name},
				"ports":    ports,
			},
		}
		item, err := marshalReleaseRenderedObject("Service", service.Name, namespace, obj)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
		if i == 0 {
			switch model.ReleaseStrategyToType(release.Strategy) {
			case model.BlueGreen:
				preview, err := buildDerivedReleaseServiceResource(namespace, service, service.Name+"-preview")
				if err != nil {
					return nil, err
				}
				out = append(out, preview)
			case model.Canary:
				canary, err := buildDerivedReleaseServiceResource(namespace, service, service.Name+"-canary")
				if err != nil {
					return nil, err
				}
				out = append(out, canary)
			}
		}
	}
	return out, nil
}

func buildDerivedReleaseServiceResource(namespace string, service manifestdomain.ManifestService, derivedName string) (model.ReleaseRenderedResource, error) {
	ports := make([]map[string]any, 0, len(service.Ports))
	for _, port := range service.Ports {
		ports = append(ports, map[string]any{
			"name":       port.Name,
			"port":       port.ServicePort,
			"targetPort": port.TargetPort,
			"protocol":   releaseDefaultProtocol(port.Protocol),
		})
	}
	metadata := map[string]any{"name": derivedName}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	obj := map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   metadata,
		"spec": map[string]any{
			"selector": map[string]any{"app.kubernetes.io/name": service.Name},
			"ports":    ports,
		},
	}
	return marshalReleaseRenderedObject("Service", derivedName, namespace, obj)
}

func buildReleaseWorkloadResource(namespace, applicationName string, manifest *manifestdomain.Manifest, release *model.Release) (model.ReleaseRenderedResource, error) {
	workload := manifest.WorkloadConfigSnapshot
	selectorName := applicationName
	if len(manifest.ServicesSnapshot) > 0 && strings.TrimSpace(manifest.ServicesSnapshot[0].Name) != "" {
		selectorName = strings.TrimSpace(manifest.ServicesSnapshot[0].Name)
	}
	labels := map[string]any{
		"app.kubernetes.io/name": selectorName,
		"devflow.application/id": release.ApplicationID.String(),
	}
	for k, v := range workload.Labels {
		if strings.TrimSpace(k) == "" {
			continue
		}
		labels[k] = v
	}
	annotations := map[string]any{}
	for k, v := range workload.Annotations {
		if strings.TrimSpace(k) == "" {
			continue
		}
		annotations[k] = v
	}
	metadata := map[string]any{
		"name":   applicationName,
		"labels": labels,
	}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	env := make([]map[string]any, 0, len(workload.Env))
	for _, entry := range workload.Env {
		env = append(env, map[string]any{"name": entry.Name, "value": entry.Value})
	}
	container := map[string]any{
		"name":                   applicationName,
		"image":                  manifest.ImageRef,
		"imagePullPolicy":        "IfNotPresent",
		"env":                    env,
		"resources":              workload.Resources,
		"terminationMessagePath": "/dev/termination-log",
		"terminationMessagePolicy": "File",
	}
	if ports := buildReleaseContainerPorts(manifest.ServicesSnapshot); len(ports) > 0 {
		container["ports"] = ports
	}
	if len(workload.Probes) > 0 {
		for k, v := range workload.Probes {
			container[k] = v
		}
	}
	if len(release.AppConfigSnapshot.Data) > 0 || len(release.AppConfigSnapshot.Files) > 0 {
		volumeName := "app-config"
		container["volumeMounts"] = []map[string]any{{
			"name":      volumeName,
			"mountPath": firstNonEmptyString(strings.TrimSpace(release.AppConfigSnapshot.MountPath), "/etc/config"),
			"readOnly":  true,
		}}
	}
	podSpec := map[string]any{
		"dnsPolicy":                 "ClusterFirst",
		"restartPolicy":             "Always",
		"schedulerName":             "default-scheduler",
		"securityContext":           map[string]any{},
		"terminationGracePeriodSeconds": 30,
		"imagePullSecrets":          []map[string]any{{"name": "aliyun-docker-config"}},
		"containers":                []map[string]any{container},
	}
	if strings.TrimSpace(workload.ServiceAccountName) != "" {
		podSpec["serviceAccount"] = workload.ServiceAccountName
		podSpec["serviceAccountName"] = workload.ServiceAccountName
	}
	if len(release.AppConfigSnapshot.Data) > 0 || len(release.AppConfigSnapshot.Files) > 0 {
		podSpec["volumes"] = []map[string]any{{
			"name": "app-config",
			"configMap": map[string]any{
				"name":        applicationName,
				"defaultMode": 420,
			},
		}}
	}
	spec := map[string]any{
		"progressDeadlineSeconds": 600,
		"revisionHistoryLimit":   10,
		"replicas":               workload.Replicas,
		"strategy": map[string]any{
			"type": "RollingUpdate",
			"rollingUpdate": map[string]any{
				"maxSurge":       "25%",
				"maxUnavailable": "25%",
			},
		},
		"selector": map[string]any{
			"matchLabels": map[string]any{"app.kubernetes.io/name": selectorName},
		},
		"template": map[string]any{
			"metadata": map[string]any{"labels": labels, "annotations": annotations},
			"spec":     podSpec,
		},
	}
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen:
		spec["strategy"] = map[string]any{
			"blueGreen": map[string]any{
				"activeService":  selectorName,
				"previewService": selectorName + "-preview",
			},
		}
		obj := map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Rollout",
			"metadata":   metadata,
			"spec":       spec,
		}
		return marshalReleaseRenderedObject("Rollout", applicationName, namespace, obj)
	case model.Canary:
		spec["strategy"] = map[string]any{
			"canary": map[string]any{
				"stableService": selectorName,
				"canaryService": selectorName + "-canary",
				"steps": []map[string]any{
					{"setWeight": 10},
					{"pause": map[string]any{}},
					{"setWeight": 30},
					{"pause": map[string]any{}},
					{"setWeight": 60},
					{"pause": map[string]any{}},
					{"setWeight": 100},
				},
			},
		}
		obj := map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Rollout",
			"metadata":   metadata,
			"spec":       spec,
		}
		return marshalReleaseRenderedObject("Rollout", applicationName, namespace, obj)
	default:
		obj := map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   metadata,
			"spec":       spec,
		}
		return marshalReleaseRenderedObject("Deployment", applicationName, namespace, obj)
	}
}

func buildReleaseContainerPorts(services []manifestdomain.ManifestService) []map[string]any {
	if len(services) == 0 {
		return nil
	}
	ports := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, service := range services {
		for _, port := range service.Ports {
			key := fmt.Sprintf("%s/%d/%s", strings.TrimSpace(port.Name), port.TargetPort, releaseDefaultProtocol(port.Protocol))
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item := map[string]any{
				"containerPort": port.TargetPort,
				"protocol":      releaseDefaultProtocol(port.Protocol),
			}
			if name := strings.TrimSpace(port.Name); name != "" {
				item["name"] = name
			}
			ports = append(ports, item)
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func buildReleaseVirtualService(namespace, applicationName string, routes []model.ReleaseRoute) map[string]any {
	if len(routes) == 0 {
		return nil
	}
	hostsSet := map[string]struct{}{}
	httpRoutes := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		if strings.TrimSpace(route.Host) != "" {
			hostsSet[strings.TrimSpace(route.Host)] = struct{}{}
		}
		match := map[string]any{}
		if strings.TrimSpace(route.Path) != "" {
			match["uri"] = map[string]any{"prefix": route.Path}
		}
		httpRoutes = append(httpRoutes, map[string]any{
			"match": []map[string]any{match},
			"route": []map[string]any{{
				"destination": map[string]any{
					"host": route.ServiceName,
					"port": map[string]any{
						"number": route.ServicePort,
					},
				},
			}},
		})
	}
	hosts := make([]string, 0, len(hostsSet))
	for host := range hostsSet {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	metadata := map[string]any{"name": applicationName}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	return map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata":   metadata,
		"spec": map[string]any{
			"hosts": hosts,
			"http":  httpRoutes,
		},
	}
}

func marshalReleaseRenderedObject(kind, name, namespace string, object any) (model.ReleaseRenderedResource, error) {
	body, err := yaml.Marshal(object)
	if err != nil {
		return model.ReleaseRenderedResource{}, fmt.Errorf("marshal %s %s: %w", kind, name, err)
	}
	return model.ReleaseRenderedResource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		YAML:      string(body),
		Object:    object.(map[string]any),
	}, nil
}

func deriveReleaseApplicationName(manifest *manifestdomain.Manifest) string {
	if manifest == nil {
		return ""
	}
	if len(manifest.ServicesSnapshot) > 0 && strings.TrimSpace(manifest.ServicesSnapshot[0].Name) != "" {
		return strings.TrimSpace(manifest.ServicesSnapshot[0].Name)
	}
	return manifest.ApplicationID.String()
}

func releaseDefaultProtocol(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "TCP"
	}
	return trimmed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func releaseBundleDigest(bundle *model.ReleaseBundle) string {
	if bundle == nil {
		return ""
	}
	content := releaseBundleCombinedContent(bundle)
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func releaseBundleCombinedContent(bundle *model.ReleaseBundle) string {
	if bundle == nil {
		return ""
	}
	for _, file := range bundle.Files {
		if file.Path == "bundle.yaml" && strings.TrimSpace(file.Content) != "" {
			return file.Content
		}
	}
	parts := make([]string, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		if strings.TrimSpace(file.Content) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(file.Content))
	}
	if len(parts) == 0 {
		for _, object := range bundle.RenderedObjects {
			if strings.TrimSpace(object.YAML) == "" {
				continue
			}
			parts = append(parts, strings.TrimSpace(object.YAML))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n---\n") + "\n"
}
