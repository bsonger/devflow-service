package service

import (
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"sigs.k8s.io/yaml"
)

var ErrManifestImageNotDeployable = sharederrs.FailedPrecondition("image has neither digest nor tag")

// resolveWorkloadImageRef resolves the workload image reference used by both the manifest inspection view and the release deployable bundle.
// The shared image ref does not mean the rendered outputs are interchangeable: manifest resources stay inspection-only while release bundles add deploy-time state.
func resolveWorkloadImageRef(repository, tag, digest string) (string, map[string]string, error) {
	annotations := map[string]string{}
	if digest != "" {
		if tag != "" {
			annotations["devflow.io/image-tag"] = tag
			annotations["devflow.io/image-ref"] = repository + ":" + tag
		}
		return repository + "@" + digest, annotations, nil
	}
	if tag != "" {
		return repository + ":" + tag, annotations, nil
	}
	return "", nil, ErrManifestImageNotDeployable
}

// renderManifestResources builds a manifest-owned derived resources view for inspection.
// It intentionally renders only from manifest-frozen snapshots plus the resolved workload image,
// and does not include release-time inputs such as environment binding, app config, routes, or bundle publication metadata.
func renderManifestResources(namespace, applicationName, applicationId string, workload manifestdomain.ManifestWorkloadConfig, services []manifestdomain.ManifestService, imageRef string, annotations map[string]string) ([]manifestdomain.ManifestRenderedResource, error) {
	objects := make([]manifestdomain.ManifestRenderedResource, 0, len(services)+1)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
	}
	workloadLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
		"devflow.application/id": applicationId,
	}
	for k, v := range workload.Labels {
		if strings.TrimSpace(k) == "" {
			continue
		}
		workloadLabels[k] = v
	}

	for _, service := range services {
		ports := make([]map[string]any, 0, len(service.Ports))
		for _, port := range service.Ports {
			ports = append(ports, map[string]any{
				"name":       port.Name,
				"port":       port.ServicePort,
				"targetPort": port.TargetPort,
				"protocol":   defaultProtocol(port.Protocol),
			})
		}
		metadata := map[string]any{
			"name": service.Name,
		}
		if namespace != "" {
			metadata["namespace"] = namespace
		}
		serviceObj := map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   metadata,
			"spec": map[string]any{
				"selector": selectorLabels,
				"ports":    ports,
			},
		}
		item, err := marshalRenderedObject("Service", service.Name, namespace, serviceObj)
		if err != nil {
			return nil, err
		}
		objects = append(objects, item)
	}

	env := make([]map[string]any, 0, len(workload.Env))
	for _, entry := range workload.Env {
		env = append(env, map[string]any{"name": entry.Name, "value": entry.Value})
	}
	templateAnnotations := map[string]any{}
	for k, v := range annotations {
		templateAnnotations[k] = v
	}
	for k, v := range workload.Annotations {
		if strings.TrimSpace(k) == "" {
			continue
		}
		templateAnnotations[k] = v
	}
	deploymentMetadata := map[string]any{
		"name":        applicationName,
		"labels":      workloadLabels,
		"annotations": templateAnnotations,
	}
	if namespace != "" {
		deploymentMetadata["namespace"] = namespace
	}
	container := map[string]any{
		"name":      applicationName,
		"image":     imageRef,
		"env":       env,
		"resources": buildKubernetesResourceRequirements(workload.Resources),
	}
	applyWorkloadProbes(container, workload.Probes)
	deploymentObj := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   deploymentMetadata,
		"spec": map[string]any{
			"replicas": workload.Replicas,
			"selector": map[string]any{
				"matchLabels": selectorLabels,
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"labels":      workloadLabels,
					"annotations": templateAnnotations,
				},
				"spec": map[string]any{
					"imagePullSecrets": []map[string]any{{"name": "aliyun-docker-config"}},
					"containers":       []map[string]any{container},
				},
			},
		},
	}
	if strings.TrimSpace(workload.ServiceAccountName) != "" {
		deploymentObj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["serviceAccountName"] = workload.ServiceAccountName
	}
	item, err := marshalRenderedObject("Deployment", applicationName, namespace, deploymentObj)
	if err != nil {
		return nil, err
	}
	objects = append(objects, item)
	return objects, nil
}

func buildKubernetesResourceRequirements(resources workloadconfigdomain.WorkloadResourceRequirements) map[string]any {
	expanded := expandWorkloadResourceRequirements(resources)
	out := map[string]any{}
	if requests := buildKubernetesResourceList(expanded.Requests); len(requests) > 0 {
		out["requests"] = requests
	}
	if limits := buildKubernetesResourceList(expanded.Limits); len(limits) > 0 {
		out["limits"] = limits
	}
	return out
}

func expandWorkloadResourceRequirements(resources workloadconfigdomain.WorkloadResourceRequirements) workloadconfigdomain.WorkloadResourceRequirements {
	if mapped, ok := workloadconfigdomain.WorkloadSizeClassResources[resources.SizeClass]; ok {
		return mapped
	}
	return resources
}

func buildKubernetesResourceList(resources workloadconfigdomain.WorkloadResourceList) map[string]any {
	out := map[string]any{}
	if cpu := strings.TrimSpace(resources.CPU); cpu != "" {
		out["cpu"] = cpu
	}
	if memory := strings.TrimSpace(resources.Memory); memory != "" {
		out["memory"] = memory
	}
	return out
}

func applyWorkloadProbes(container map[string]any, probes workloadconfigdomain.WorkloadProbes) {
	if probe := buildKubernetesProbe(probes.Liveness); len(probe) > 0 {
		container["livenessProbe"] = probe
	}
	if probe := buildKubernetesProbe(probes.Readiness); len(probe) > 0 {
		container["readinessProbe"] = probe
	}
	if probe := buildKubernetesProbe(probes.Startup); len(probe) > 0 {
		container["startupProbe"] = probe
	}
}

func buildKubernetesProbe(probe *workloadconfigdomain.WorkloadProbe) map[string]any {
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

func marshalRenderedObject(kind, name, namespace string, object any) (manifestdomain.ManifestRenderedResource, error) {
	body, err := yaml.Marshal(object)
	if err != nil {
		return manifestdomain.ManifestRenderedResource{}, fmt.Errorf("marshal %s %s: %w", kind, name, err)
	}
	return manifestdomain.ManifestRenderedResource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		YAML:      string(body),
		Object:    object.(map[string]any),
	}, nil
}

func defaultProtocol(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "TCP"
	}
	return trimmed
}
