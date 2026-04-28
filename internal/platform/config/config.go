package config

import (
	"context"
	"database/sql"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	runtimeobserver "github.com/bsonger/devflow-service/internal/runtime/observer"
	runtimehttp "github.com/bsonger/devflow-service/internal/runtime/transport/http"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
)

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
	Endpoint           string  `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	Protocol           string  `mapstructure:"protocol" json:"protocol" yaml:"protocol"`
	ServiceName        string  `mapstructure:"service_name" json:"service_name" yaml:"service_name"`
	ResourceAttributes string  `mapstructure:"resource_attributes" json:"resource_attributes" yaml:"resource_attributes"`
	SampleRatio        float64 `mapstructure:"sample_ratio" json:"sample_ratio" yaml:"sample_ratio"`
}

type DownstreamConfig struct {
	ReleaseServiceBaseURL string `mapstructure:"release_service_base_url" json:"release_service_base_url" yaml:"release_service_base_url"`
}

type ObserverConfig struct {
	SharedToken         string `mapstructure:"shared_token" json:"shared_token" yaml:"shared_token"`
	TektonNamespace     string `mapstructure:"tekton_namespace" json:"tekton_namespace" yaml:"tekton_namespace"`
	PollIntervalSeconds int    `mapstructure:"poll_interval_seconds" json:"poll_interval_seconds" yaml:"poll_interval_seconds"`
}

type Config struct {
	Server     *ServerConfig     `mapstructure:"server" json:"server" yaml:"server"`
	Postgres   *PostgresConfig   `mapstructure:"postgres" json:"postgres" yaml:"postgres"`
	Log        *LogConfig        `mapstructure:"log" json:"log" yaml:"log"`
	Otel       *OtelConfig       `mapstructure:"otel" json:"otel" yaml:"otel"`
	Downstream *DownstreamConfig `mapstructure:"downstream" json:"downstream" yaml:"downstream"`
	Observer   *ObserverConfig   `mapstructure:"observer" json:"observer" yaml:"observer"`
	Pyroscope  string            `mapstructure:"pyroscope" json:"pyroscope" yaml:"pyroscope"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath("./config/")
	v.AddConfigPath("/etc/devflow/config/")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var config *Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func InitConfig(ctx context.Context, config *Config) error {
	_, err := InitRuntime(ctx, config, "")
	return err
}

func InitRuntime(ctx context.Context, config *Config, serviceName string) (func(context.Context) error, error) {
	shutdown, err := observability.Init(ctx, observability.RuntimeOptions{
		LogLevel:               stringValue(config.Log, func(v *LogConfig) string { return v.Level }),
		LogFormat:              stringValue(config.Log, func(v *LogConfig) string { return v.Format }),
		OtelEndpoint:           stringValue(config.Otel, func(v *OtelConfig) string { return v.Endpoint }),
		OtelProtocol:           stringValue(config.Otel, func(v *OtelConfig) string { return v.Protocol }),
		OtelService:            stringValue(config.Otel, func(v *OtelConfig) string { return v.ServiceName }),
		OtelResourceAttributes: stringValue(config.Otel, func(v *OtelConfig) string { return v.ResourceAttributes }),
		OtelSampleRatio:        floatValue(config.Otel, func(v *OtelConfig) float64 { return v.SampleRatio }),
		PyroscopeAddr:          configValue(config, func(v *Config) string { return v.Pyroscope }),
		ServiceOverride:        serviceName,
	})
	if err != nil {
		return nil, err
	}

	conn, err := sql.Open("pgx", stringValue(config.Postgres, func(v *PostgresConfig) string { return v.DSN }))
	if err != nil {
		return shutdown, err
	}
	db.ApplyPool(conn,
		intValue(config.Postgres, func(v *PostgresConfig) int { return v.MaxOpenConns }),
		intValue(config.Postgres, func(v *PostgresConfig) int { return v.MaxIdleConns }),
		intValue(config.Postgres, func(v *PostgresConfig) int { return v.ConnMaxLifetimeMinutes }),
	)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return shutdown, err
	}

	db.InitPostgres(conn)

	if err := startTektonManifestObserver(ctx, config); err != nil {
		_ = conn.Close()
		return shutdown, err
	}
	return func(shutdownCtx context.Context) error {
		closeErr := conn.Close()
		shutdownErr := shutdown(shutdownCtx)
		if shutdownErr != nil {
			return shutdownErr
		}
		return closeErr
	}, nil
}

func intValue[T any](value *T, getter func(*T) int) int {
	if value == nil {
		return 0
	}
	return getter(value)
}

func floatValue[T any](value *T, getter func(*T) float64) float64 {
	if value == nil {
		return 0
	}
	return getter(value)
}

func stringValue[T any](value *T, getter func(*T) string) string {
	if value == nil {
		return ""
	}
	return getter(value)
}

func configValue(cfg *Config, getter func(*Config) string) string {
	if cfg == nil {
		return ""
	}
	return getter(cfg)
}

func startTektonManifestObserver(ctx context.Context, config *Config) error {
	runtimehttp.ObserverSharedToken = stringValue(config.Observer, func(v *ObserverConfig) string { return v.SharedToken })
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil
	}
	return runtimeobserver.StartTektonManifestObserver(ctx, restCfg, runtimeobserver.TektonManifestObserverConfig{
		Enabled:               true,
		TektonNamespace:       stringValue(config.Observer, func(v *ObserverConfig) string { return v.TektonNamespace }),
		PollInterval:          time.Duration(intValue(config.Observer, func(v *ObserverConfig) int { return v.PollIntervalSeconds })) * time.Second,
		ReleaseServiceBaseURL: stringValue(config.Downstream, func(v *DownstreamConfig) string { return v.ReleaseServiceBaseURL }),
		ObserverToken:         stringValue(config.Observer, func(v *ObserverConfig) string { return v.SharedToken }),
	})
}
