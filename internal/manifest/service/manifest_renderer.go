package service

import (
	"fmt"
	"path/filepath"
	"sort"
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

func joinRenderedYAML(objects []manifestdomain.ManifestRenderedObject) string {
	parts := make([]string, 0, len(objects))
	for _, item := range objects {
		if trimmed := strings.TrimSpace(item.YAML); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n---\n")
}

func renderManifestObjects(namespace, applicationName, applicationID, environmentID, configMapName string, appConfig manifestdomain.ManifestAppConfig, workload manifestdomain.ManifestWorkloadConfig, services []manifestdomain.ManifestService, routes []manifestdomain.ManifestRoute, imageRef string, annotations map[string]string) ([]manifestdomain.ManifestRenderedObject, error) {
	objects := make([]manifestdomain.ManifestRenderedObject, 0, len(services)+3)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
	}
	workloadLabels := map[string]string{
		"app.kubernetes.io/name": applicationName,
		"devflow.application/id": applicationID,
		"devflow.environment/id": environmentID,
	}

	configFiles := manifestConfigFiles(appConfig)
	configMapData, volumeItems := configMapVolumeEntries(configFiles)
	mountPath, subPath := resolveAppConfigMount(appConfig, configFiles)
	configMap := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      configMapName,
			"namespace": namespace,
		},
		"data": configMapData,
	}
	item, err := marshalRenderedObject("ConfigMap", configMapName, namespace, configMap)
	if err != nil {
		return nil, err
	}
	objects = append(objects, item)

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
		serviceObj := map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      service.Name,
				"namespace": namespace,
			},
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

	httpRoutes := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		httpRoute := map[string]any{
			"name": route.Name,
			"match": []map[string]any{{
				"uri": map[string]any{"prefix": defaultPath(route.Path)},
			}},
			"route": []map[string]any{{
				"destination": map[string]any{
					"host": route.ServiceName,
					"port": map[string]any{"number": route.ServicePort},
				},
			}},
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}
	hosts := uniqueHosts(routes)
	if len(hosts) > 0 && len(httpRoutes) > 0 {
		virtualServiceObj := map[string]any{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualService",
			"metadata": map[string]any{
				"name":      applicationName,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"hosts": hosts,
				"http":  httpRoutes,
			},
		}
		item, err = marshalRenderedObject("VirtualService", applicationName, namespace, virtualServiceObj)
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
	volumeMount := map[string]any{
		"name":      "config",
		"mountPath": mountPath,
		"readOnly":  true,
	}
	if subPath != "" {
		volumeMount["subPath"] = subPath
	}
	configVolume := map[string]any{
		"name": "config",
		"configMap": map[string]any{
			"name": configMapName,
		},
	}
	if len(volumeItems) > 0 {
		configVolume["configMap"].(map[string]any)["items"] = volumeItems
	}
	deploymentObj := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      applicationName,
			"namespace": namespace,
			"labels":    workloadLabels,
		},
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
						"name":         applicationName,
						"image":        imageRef,
						"env":          env,
						"resources":    workload.Resources,
						"volumeMounts": []map[string]any{volumeMount},
					}},
					"volumes": []map[string]any{configVolume},
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
	item, err = marshalRenderedObject("Deployment", applicationName, namespace, deploymentObj)
	if err != nil {
		return nil, err
	}
	objects = append(objects, item)
	return objects, nil
}

func marshalRenderedObject(kind, name, namespace string, object any) (manifestdomain.ManifestRenderedObject, error) {
	body, err := yaml.Marshal(object)
	if err != nil {
		return manifestdomain.ManifestRenderedObject{}, fmt.Errorf("marshal %s %s: %w", kind, name, err)
	}
	return manifestdomain.ManifestRenderedObject{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		YAML:      string(body),
	}, nil
}

func manifestConfigFiles(appConfig manifestdomain.ManifestAppConfig) []manifestdomain.ManifestFile {
	if len(appConfig.Files) > 0 {
		out := make([]manifestdomain.ManifestFile, len(appConfig.Files))
		copy(out, appConfig.Files)
		return out
	}
	if len(appConfig.Data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(appConfig.Data))
	for key := range appConfig.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]manifestdomain.ManifestFile, 0, len(keys))
	for _, key := range keys {
		out = append(out, manifestdomain.ManifestFile{Name: key, Content: appConfig.Data[key]})
	}
	return out
}

func configMapVolumeEntries(files []manifestdomain.ManifestFile) (map[string]string, []map[string]any) {
	data := make(map[string]string, len(files))
	items := make([]map[string]any, 0, len(files))
	for index, file := range files {
		key := fmt.Sprintf("config-%d", index)
		path := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(file.Name)), "./")
		if path == "" {
			path = fmt.Sprintf("config-%d.yaml", index)
		}
		data[key] = file.Content
		items = append(items, map[string]any{
			"key":  key,
			"path": path,
		})
	}
	return data, items
}

func resolveAppConfigMount(appConfig manifestdomain.ManifestAppConfig, files []manifestdomain.ManifestFile) (string, string) {
	mountPath := strings.TrimSpace(appConfig.MountPath)
	if mountPath == "" {
		mountPath = "/etc/devflow/config"
	}
	normalized := strings.ToLower(mountPath)
	if len(files) == 1 && (strings.HasSuffix(normalized, ".yaml") || strings.HasSuffix(normalized, ".yml") || strings.HasSuffix(normalized, ".json")) {
		subPath := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(files[0].Name)), "./")
		if subPath == "" {
			subPath = defaultConfigMountFile(files[0].Name)
		}
		return mountPath, subPath
	}
	return mountPath, ""
}

func defaultConfigMountFile(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "configuration.yaml"
	}
	return base
}

func uniqueHosts(routes []manifestdomain.ManifestRoute) []string {
	seen := make(map[string]struct{}, len(routes))
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		if route.Host == "" {
			continue
		}
		if _, ok := seen[route.Host]; ok {
			continue
		}
		seen[route.Host] = struct{}{}
		out = append(out, route.Host)
	}
	return out
}

func defaultPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "/"
	}
	return path
}

func defaultProtocol(protocol string) string {
	if strings.TrimSpace(protocol) == "" {
		return "TCP"
	}
	return protocol
}
