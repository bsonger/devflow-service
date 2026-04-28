package downstream

import (
	"context"
	"fmt"
	"net/url"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ManifestFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type AppConfig struct {
	ID                string            `json:"id"`
	ApplicationID     string            `json:"application_id"`
	EnvironmentID     string            `json:"environment_id"`
	Name              string            `json:"name"`
	MountPath         string            `json:"mount_path,omitempty"`
	SourcePath        string            `json:"source_path"`
	Files             []ManifestFile    `json:"files,omitempty"`
	RenderedConfigMap map[string]string `json:"rendered_configmap,omitempty"`
	SourceCommit      string            `json:"source_commit,omitempty"`
}

type renderedConfigMapEnvelope struct {
	Data map[string]string `json:"data,omitempty"`
}

type WorkloadConfig struct {
	ID            string         `json:"id"`
	ApplicationID string         `json:"application_id"`
	Name          string         `json:"name"`
	Replicas      int            `json:"replicas"`
	Resources     map[string]any `json:"resources,omitempty"`
	Probes        map[string]any `json:"probes,omitempty"`
	Env           []EnvVar       `json:"env,omitempty"`
	WorkloadType  string         `json:"workload_type,omitempty"`
	Strategy      string         `json:"strategy,omitempty"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) FindAppConfig(ctx context.Context, applicationId, environmentId string) (*AppConfig, error) {
	item, err := c.findAppConfigMetadata(ctx, applicationId, environmentId)
	if err != nil {
		return nil, err
	}
	if item.ID == "" {
		return nil, nil
	}
	return c.GetAppConfig(ctx, item.ID)
}

func (c *Client) GetAppConfig(ctx context.Context, id string) (*AppConfig, error) {
	var item struct {
		ID                string                    `json:"id"`
		ApplicationID     string                    `json:"application_id"`
		EnvironmentID     string                    `json:"environment_id"`
		Name              string                    `json:"name"`
		MountPath         string                    `json:"mount_path,omitempty"`
		SourcePath        string                    `json:"source_path"`
		Files             []ManifestFile            `json:"files,omitempty"`
		RenderedConfigMap renderedConfigMapEnvelope `json:"rendered_configmap,omitempty"`
		SourceCommit      string                    `json:"source_commit,omitempty"`
	}
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/app-configs/%s", id), &item); err != nil {
		return nil, err
	}
	return &AppConfig{
		ID:                item.ID,
		ApplicationID:     item.ApplicationID,
		EnvironmentID:     item.EnvironmentID,
		Name:              item.Name,
		MountPath:         item.MountPath,
		SourcePath:        item.SourcePath,
		Files:             item.Files,
		RenderedConfigMap: item.RenderedConfigMap.Data,
		SourceCommit:      item.SourceCommit,
	}, nil
}

func (c *Client) FindWorkloadConfig(ctx context.Context, applicationId string) (*WorkloadConfig, error) {
	item, err := c.findWorkloadConfigMetadata(ctx, applicationId)
	if err != nil {
		return nil, err
	}
	if item.ID == "" {
		return nil, nil
	}
	return c.GetWorkloadConfig(ctx, item.ID)
}

func (c *Client) findAppConfigMetadata(ctx context.Context, applicationId, environmentId string) (AppConfig, error) {
	path := fmt.Sprintf("/api/v1/app-configs?application_id=%s&environment_id=%s", url.QueryEscape(applicationId), url.QueryEscape(environmentId))
	var items []AppConfig
	if err := c.GetEnvelopeData(ctx, path, &items); err != nil {
		return AppConfig{}, err
	}
	if len(items) > 0 {
		item := items[0]
		if item.ID != "" {
			return item, nil
		}
	}
	return AppConfig{}, nil
}

func (c *Client) GetWorkloadConfig(ctx context.Context, id string) (*WorkloadConfig, error) {
	var item WorkloadConfig
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/workload-configs/%s", id), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *Client) findWorkloadConfigMetadata(ctx context.Context, applicationId string) (WorkloadConfig, error) {
	path := fmt.Sprintf("/api/v1/workload-configs?application_id=%s", url.QueryEscape(applicationId))
	var fallback []WorkloadConfig
	if err := c.GetEnvelopeData(ctx, path, &fallback); err != nil {
		return WorkloadConfig{}, err
	}
	if len(fallback) == 0 {
		return WorkloadConfig{}, nil
	}
	return fallback[0], nil
}
