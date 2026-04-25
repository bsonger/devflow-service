package domain

import "k8s.io/client-go/rest"

var KubeConfig *rest.Config

type Consul struct {
	Address string `mapstructure:"address" json:"address" yaml:"address"`
	Key     string `mapstructure:"key" json:"key" yaml:"key"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" json:"level" yaml:"level"`
	Format string `mapstructure:"format" json:"format" yaml:"format"`
}

type ServerConfig struct {
	Port int `mapstructure:"port" json:"port" yaml:"port"`
}

type PostgresConfig struct {
	DSN                    string `mapstructure:"dsn" json:"dsn" yaml:"dsn"`
	MaxOpenConns           int    `mapstructure:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int    `mapstructure:"conn_max_lifetime_minutes" json:"conn_max_lifetime_minutes" yaml:"conn_max_lifetime_minutes"`
}

type OtelConfig struct {
	Endpoint    string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	ServiceName string `mapstructure:"service_name" json:"service_name" yaml:"service_name"`
}

type Repo struct {
	Address string `mapstructure:"address" json:"address" yaml:"address"`
	Path    string `mapstructure:"path" json:"path" yaml:"path"`
}

type RuntimeServiceConfig struct {
	BaseURL string `mapstructure:"base_url" json:"base_url" yaml:"base_url"`
}

type ObserverConfig struct {
	SharedToken string `mapstructure:"shared_token" json:"shared_token" yaml:"shared_token"`
}

type DownstreamConfig struct {
	PlatformOrchestratorBaseURL string `mapstructure:"platform_orchestrator_base_url" json:"platform_orchestrator_base_url" yaml:"platform_orchestrator_base_url"`
	AppServiceBaseURL           string `mapstructure:"app_service_base_url" json:"app_service_base_url" yaml:"app_service_base_url"`
	NetworkServiceBaseURL       string `mapstructure:"network_service_base_url" json:"network_service_base_url" yaml:"network_service_base_url"`
	ConfigServiceBaseURL        string `mapstructure:"config_service_base_url" json:"config_service_base_url" yaml:"config_service_base_url"`
}

type ImageRegistryRuntimeConfig struct {
	Registry  string `mapstructure:"registry" json:"registry" yaml:"registry"`
	Namespace string `mapstructure:"namespace" json:"namespace" yaml:"namespace"`
	Username  string `mapstructure:"username" json:"username" yaml:"username"`
	Password  string `mapstructure:"password" json:"password" yaml:"password"`
}

type ManifestRegistryRuntimeConfig struct {
	Registry   string `mapstructure:"registry" json:"registry" yaml:"registry"`
	Namespace  string `mapstructure:"namespace" json:"namespace" yaml:"namespace"`
	Repository string `mapstructure:"repository" json:"repository" yaml:"repository"`
	Username   string `mapstructure:"username" json:"username" yaml:"username"`
	Password   string `mapstructure:"password" json:"password" yaml:"password"`
	PlainHTTP  bool   `mapstructure:"plain_http" json:"plain_http" yaml:"plain_http"`
}
