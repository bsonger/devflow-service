package service

import (
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"sigs.k8s.io/yaml"
)

var ErrManifestImageNotDeployable = sharederrs.FailedPrecondition("image has neither digest nor tag")

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

func renderManifestResources(namespace, applicationName, applicationId string, workload manifestdomain.ManifestWorkloadConfig, services []manifestdomain.ManifestService, imageRef string, annotations map[string]string) ([]manifestdomain.ManifestRenderedResource, error) {
	objects := make([]manifestdomain.ManifestRenderedResource, 0, len(services)+1)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
	}
	workloadLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
		"devflow.application/id": applicationId,
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
	deploymentMetadata := map[string]any{
		"name":   applicationName,
		"labels": workloadLabels,
	}
	if namespace != "" {
		deploymentMetadata["namespace"] = namespace
	}
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
					"containers": []map[string]any{{
						"name":      applicationName,
						"image":     imageRef,
						"env":       env,
						"resources": workload.Resources,
					}},
				},
			},
		},
	}
	if len(workload.Probes) > 0 {
		container := deploymentObj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]map[string]any)[0]
		for k, v := range workload.Probes {
			container[k] = v
		}
	}
	item, err := marshalRenderedObject("Deployment", applicationName, namespace, deploymentObj)
	if err != nil {
		return nil, err
	}
	objects = append(objects, item)
	return objects, nil
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
