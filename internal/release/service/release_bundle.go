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
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"sigs.k8s.io/yaml"
)

const defaultOTELServiceNamespace = "devflow"

const (
	releaseMetricsPortName      = "metrics"
	releaseMetricsPortEnv       = "METRICS_PORT"
	releaseScrapeLabel          = "observability.devflow.io/scrape"
	releaseScrapeProfileLabel   = "observability.devflow.io/scrape-profile"
	releaseDefaultScrapeProfile = "default"
)

// buildReleaseBundle materializes the release-owned deployable bundle from manifest-frozen inputs plus release-time freeze inputs.
// Unlike manifest resource inspection views, this output is the publishable deployment payload used for bundle preview, OCI publication, and Argo delivery.
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

	serviceResources, err := buildReleaseServiceResources(namespace, manifest, release)
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

func buildReleaseServiceResources(namespace string, manifest *manifestdomain.Manifest, release *model.Release) ([]model.ReleaseRenderedResource, error) {
	services := manifest.ServicesSnapshot
	extras := 0
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen, model.Canary:
		extras = 1
	}
	out := make([]model.ReleaseRenderedResource, 0, len(services)+extras)
	metrics := releaseWorkloadMetrics(manifest)
	for i, service := range services {
		ports := buildReleaseServicePorts(service, i == 0, metrics)
		metadata := map[string]any{"name": service.Name}
		if labels := buildReleaseServiceLabels(i == 0, metrics); len(labels) > 0 {
			metadata["labels"] = labels
		}
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
	labels := releaseWorkloadLabels(selectorName, workload.Labels, release)
	annotations := releaseSupplementaryAnnotations(workload.Annotations)
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
	env := buildReleaseWorkloadEnv(applicationName, manifest, release)
	container := map[string]any{
		"name":                     applicationName,
		"image":                    manifest.ImageRef,
		"imagePullPolicy":          "IfNotPresent",
		"env":                      env,
		"resources":                buildReleaseKubernetesResourceRequirements(workload.Resources),
		"terminationMessagePath":   "/dev/termination-log",
		"terminationMessagePolicy": "File",
	}
	if ports := buildReleaseContainerPorts(manifest.ServicesSnapshot, workload.Metrics); len(ports) > 0 {
		container["ports"] = ports
	}
	applyReleaseWorkloadProbes(container, workload.Probes)
	if len(release.AppConfigSnapshot.Data) > 0 || len(release.AppConfigSnapshot.Files) > 0 {
		volumeName := "app-config"
		container["volumeMounts"] = []map[string]any{{
			"name":      volumeName,
			"mountPath": firstNonEmptyString(strings.TrimSpace(release.AppConfigSnapshot.MountPath), "/etc/config"),
			"readOnly":  true,
		}}
	}
	podSpec := map[string]any{
		"dnsPolicy":                     "ClusterFirst",
		"restartPolicy":                 "Always",
		"schedulerName":                 "default-scheduler",
		"securityContext":               map[string]any{},
		"terminationGracePeriodSeconds": 30,
		"imagePullSecrets":              []map[string]any{{"name": "aliyun-docker-config"}},
		"containers":                    []map[string]any{container},
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
		"revisionHistoryLimit":    10,
		"replicas":                workload.Replicas,
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

func buildReleaseWorkloadEnv(applicationName string, manifest *manifestdomain.Manifest, release *model.Release) []map[string]any {
	baseEnv := workloadEnv(manifest)
	env := make([]map[string]any, 0, len(baseEnv)+7)
	seen := map[string]struct{}{}
	appendEnv := func(name, value string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		env = append(env, map[string]any{"name": name, "value": value})
	}

	for _, entry := range baseEnv {
		appendEnv(entry.Name, entry.Value)
	}

	serviceName := strings.TrimSpace(applicationName)
	deploymentEnvironment := strings.TrimSpace(firstNonEmptyString(releaseEnvironmentID(release), "unknown"))
	serviceVersion := strings.TrimSpace(releaseServiceVersion(manifest))

	appendEnv("SERVICE_NAME", serviceName)
	appendEnv("OTEL_SERVICE_NAME", serviceName)
	appendEnv("OTEL_SERVICE_NAMESPACE", defaultOTELServiceNamespace)
	appendEnv("DEPLOYMENT_ENVIRONMENT", deploymentEnvironment)
	appendEnv("SERVICE_VERSION", serviceVersion)
	appendEnv("OTEL_RESOURCE_ATTRIBUTES", "service.namespace=$(OTEL_SERVICE_NAMESPACE),service.version=$(SERVICE_VERSION),deployment.environment.name=$(DEPLOYMENT_ENVIRONMENT)")
	if metrics := releaseWorkloadMetrics(manifest); metrics.Enabled && metrics.Port > 0 {
		appendEnv(releaseMetricsPortEnv, fmt.Sprintf("%d", metrics.Port))
	}

	return env
}

func workloadEnv(manifest *manifestdomain.Manifest) []model.EnvVar {
	if manifest == nil {
		return nil
	}
	return manifest.WorkloadConfigSnapshot.Env
}

func releaseWorkloadMetrics(manifest *manifestdomain.Manifest) workloadconfigdomain.WorkloadMetrics {
	if manifest == nil {
		return workloadconfigdomain.WorkloadMetrics{}
	}
	return manifest.WorkloadConfigSnapshot.Metrics
}

func releaseEnvironmentID(release *model.Release) string {
	if release == nil {
		return ""
	}
	return strings.TrimSpace(release.EnvironmentID)
}

func releaseServiceVersion(manifest *manifestdomain.Manifest) string {
	if manifest == nil {
		return "unknown"
	}
	if digest := strings.TrimSpace(manifest.ImageDigest); digest != "" {
		return digest
	}
	if imageRef := strings.TrimSpace(manifest.ImageRef); imageRef != "" {
		if _, digest, ok := strings.Cut(imageRef, "@"); ok && strings.TrimSpace(digest) != "" {
			return strings.TrimSpace(digest)
		}
	}
	if commitHash := strings.TrimSpace(manifest.CommitHash); commitHash != "" {
		return commitHash
	}
	return "unknown"
}

func buildReleaseKubernetesResourceRequirements(resources workloadconfigdomain.WorkloadResourceRequirements) map[string]any {
	expanded := expandReleaseWorkloadResourceRequirements(resources)
	out := map[string]any{}
	if requests := buildReleaseKubernetesResourceList(expanded.Requests); len(requests) > 0 {
		out["requests"] = requests
	}
	if limits := buildReleaseKubernetesResourceList(expanded.Limits); len(limits) > 0 {
		out["limits"] = limits
	}
	return out
}

func expandReleaseWorkloadResourceRequirements(resources workloadconfigdomain.WorkloadResourceRequirements) workloadconfigdomain.WorkloadResourceRequirements {
	if mapped, ok := workloadconfigdomain.WorkloadSizeClassResources[resources.SizeClass]; ok {
		return mapped
	}
	return resources
}

func buildReleaseKubernetesResourceList(resources workloadconfigdomain.WorkloadResourceList) map[string]any {
	out := map[string]any{}
	if cpu := strings.TrimSpace(resources.CPU); cpu != "" {
		out["cpu"] = cpu
	}
	if memory := strings.TrimSpace(resources.Memory); memory != "" {
		out["memory"] = memory
	}
	return out
}

func applyReleaseWorkloadProbes(container map[string]any, probes workloadconfigdomain.WorkloadProbes) {
	if probe := buildReleaseKubernetesProbe(probes.Liveness); len(probe) > 0 {
		container["livenessProbe"] = probe
	}
	if probe := buildReleaseKubernetesProbe(probes.Readiness); len(probe) > 0 {
		container["readinessProbe"] = probe
	}
	if probe := buildReleaseKubernetesProbe(probes.Startup); len(probe) > 0 {
		container["startupProbe"] = probe
	}
}

func buildReleaseKubernetesProbe(probe *workloadconfigdomain.WorkloadProbe) map[string]any {
	if probe == nil {
		return nil
	}
	out := map[string]any{}
	httpGet := map[string]any{}
	if path := strings.TrimSpace(probe.Path); path != "" {
		httpGet["path"] = path
	}
	if port := strings.TrimSpace(probe.Port); port != "" {
		httpGet["port"] = port
	}
	if len(httpGet) > 0 {
		out["httpGet"] = httpGet
	}
	if probe.InitialDelaySeconds > 0 {
		out["initialDelaySeconds"] = probe.InitialDelaySeconds
	}
	if probe.PeriodSeconds > 0 {
		out["periodSeconds"] = probe.PeriodSeconds
	}
	if probe.TimeoutSeconds > 0 {
		out["timeoutSeconds"] = probe.TimeoutSeconds
	}
	if probe.FailureThreshold > 0 {
		out["failureThreshold"] = probe.FailureThreshold
	}
	return out
}

func releaseWorkloadLabels(selectorName string, workloadLabels map[string]string, release *model.Release) map[string]any {
	requiredLabels := map[string]any{
		"app.kubernetes.io/name":      selectorName,
		model.ReleaseIDLabel:          release.ID.String(),
		model.ReleaseApplicationLabel: release.ApplicationID.String(),
		model.ReleaseEnvironmentLabel: strings.TrimSpace(release.EnvironmentID),
	}
	labels := make(map[string]any, len(requiredLabels)+len(workloadLabels))
	for key, value := range workloadLabels {
		if strings.TrimSpace(key) == "" {
			continue
		}
		labels[key] = value
	}
	for key, value := range requiredLabels {
		labels[key] = value
	}
	return labels
}

var releaseDriftProneAnnotationKeys = map[string]struct{}{
	"kubectl.kubernetes.io/restartedAt": {},
}

func releaseSupplementaryAnnotations(workloadAnnotations map[string]string) map[string]any {
	annotations := make(map[string]any)
	for key, value := range workloadAnnotations {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, blocked := releaseDriftProneAnnotationKeys[trimmedKey]; blocked {
			continue
		}
		annotations[trimmedKey] = value
	}
	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

func buildReleaseContainerPorts(services []manifestdomain.ManifestService, metrics workloadconfigdomain.WorkloadMetrics) []map[string]any {
	if len(services) == 0 {
		if !metrics.Enabled || metrics.Port <= 0 {
			return nil
		}
		return []map[string]any{{
			"name":          releaseMetricsPortName,
			"containerPort": metrics.Port,
			"protocol":      "TCP",
		}}
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
	if metrics.Enabled && metrics.Port > 0 {
		key := fmt.Sprintf("%s/%d/%s", releaseMetricsPortName, metrics.Port, "TCP")
		if _, ok := seen[key]; !ok {
			ports = append(ports, map[string]any{
				"name":          releaseMetricsPortName,
				"containerPort": metrics.Port,
				"protocol":      "TCP",
			})
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func buildReleaseServicePorts(service manifestdomain.ManifestService, isPrimary bool, metrics workloadconfigdomain.WorkloadMetrics) []map[string]any {
	ports := make([]map[string]any, 0, len(service.Ports)+1)
	seen := map[string]struct{}{}
	for _, port := range service.Ports {
		key := fmt.Sprintf("%s/%d/%d/%s", strings.TrimSpace(port.Name), port.ServicePort, port.TargetPort, releaseDefaultProtocol(port.Protocol))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ports = append(ports, map[string]any{
			"name":       port.Name,
			"port":       port.ServicePort,
			"targetPort": port.TargetPort,
			"protocol":   releaseDefaultProtocol(port.Protocol),
		})
	}
	if !isPrimary || !metrics.Enabled || metrics.Port <= 0 {
		return ports
	}
	for _, existing := range ports {
		if existing["name"] == releaseMetricsPortName || (existing["port"] == metrics.Port && existing["targetPort"] == metrics.Port) {
			return ports
		}
	}
	ports = append(ports, map[string]any{
		"name":       releaseMetricsPortName,
		"port":       metrics.Port,
		"targetPort": metrics.Port,
		"protocol":   "TCP",
	})
	return ports
}

func buildReleaseServiceLabels(isPrimary bool, metrics workloadconfigdomain.WorkloadMetrics) map[string]any {
	if !isPrimary || !metrics.Enabled || metrics.Port <= 0 {
		return nil
	}
	profile := strings.TrimSpace(string(metrics.ScrapeProfile))
	if profile == "" {
		profile = releaseDefaultScrapeProfile
	}
	return map[string]any{
		releaseScrapeLabel:        "true",
		releaseScrapeProfileLabel: profile,
	}
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
