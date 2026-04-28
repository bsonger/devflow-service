package config

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	manifesthttp "github.com/bsonger/devflow-service/internal/manifest/transport/http"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	"github.com/bsonger/devflow-service/internal/release/service"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	releasehttp "github.com/bsonger/devflow-service/internal/release/transport/http"
	localtekton "github.com/bsonger/devflow-service/internal/release/transport/tekton"

	"github.com/spf13/viper"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

type Config struct {
	Server           *model.ServerConfig                  `mapstructure:"server" json:"server" yaml:"server"`
	Postgres         *model.PostgresConfig                `mapstructure:"postgres" json:"postgres" yaml:"postgres"`
	Log              *model.LogConfig                     `mapstructure:"log" json:"log" yaml:"log"`
	Otel             *model.OtelConfig                    `mapstructure:"otel" json:"otel" yaml:"otel"`
	Repo             *model.Repo                          `mapstructure:"repo" json:"repo" yaml:"repo"`
	Runtime          *model.RuntimeServiceConfig          `mapstructure:"runtime" json:"runtime" yaml:"runtime"`
	Observer         *model.ObserverConfig                `mapstructure:"observer" json:"observer" yaml:"observer"`
	Worker           *model.WorkerConfig                  `mapstructure:"worker" json:"worker" yaml:"worker"`
	Downstream       *model.DownstreamConfig              `mapstructure:"downstream" json:"downstream" yaml:"downstream"`
	ImageRegistry    *model.ImageRegistryRuntimeConfig    `mapstructure:"image_registry" json:"image_registry" yaml:"image_registry"`
	ManifestRegistry *model.ManifestRegistryRuntimeConfig `mapstructure:"manifest_registry" json:"manifest_registry" yaml:"manifest_registry"`
	Consul           *model.Consul                        `mapstructure:"consul" json:"consul" yaml:"consul"`
	Pyroscope        string                               `mapstructure:"pyroscope" json:"pyroscope" yaml:"pyroscope"`
}

func Load() (*Config, error) {
	v := viper.New()
	//v.SetConfigName("config")
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
	var err error
	model.KubeConfig, err = LoadKubeConfig()
	if err != nil {
		return nil, err
	}
	//err = consul.InitConsulClient(config.Consul)
	//if err != nil {
	//	return nil, err
	//}
	//consul.LoadConsulConfigAndMerge(config.Consul)

	return config, nil
}

func InitConfig(ctx context.Context, config *Config) error {
	_, err := InitRuntime(ctx, config, "")
	return err
}

func InitRuntime(ctx context.Context, config *Config, serviceName string) (func(context.Context) error, error) {
	shutdown, err := initObservability(ctx, config.Log, config.Otel, config.Pyroscope, serviceName)
	if err != nil {
		return nil, err
	}
	runtimeCtx, runtimeCancel := context.WithCancel(ctx)

	db, err := sql.Open("pgx", stringValue(config.Postgres, func(v *model.PostgresConfig) string { return v.DSN }))
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	store.ApplyPool(db,
		intValue(config.Postgres, func(v *model.PostgresConfig) int { return v.MaxOpenConns }),
		intValue(config.Postgres, func(v *model.PostgresConfig) int { return v.MaxIdleConns }),
		intValue(config.Postgres, func(v *model.PostgresConfig) int { return v.ConnMaxLifetimeMinutes }),
	)
	if err := db.PingContext(ctx); err != nil {
		runtimeCancel()
		_ = db.Close()
		return shutdown, err
	}
	store.InitPostgres(db)
	kubeconfig, err := LoadKubeConfig()
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	err = localtekton.InitClient(ctx, kubeconfig, logger.Logger)
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	err = argoclient.Init(kubeconfig)
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	imageRegistryCfg, err := runtime.ImageRegistryConfigFromConfig(config.ImageRegistry)
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	manifestRegistryCfg, manifestRegistryEnabled, err := runtime.ManifestRegistryConfigFromConfig(config.ManifestRegistry, config.ImageRegistry)
	if err != nil {
		runtimeCancel()
		return shutdown, err
	}
	observerToken := stringValue(config.Observer, func(v *model.ObserverConfig) string { return v.SharedToken })
	releasehttp.ObserverSharedToken = observerToken
	manifesthttp.ManifestObserverSharedToken = observerToken
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{
		ImageRegistry:           imageRegistryCfg,
		ManifestRegistry:        manifestRegistryCfg,
		ManifestRegistryEnabled: manifestRegistryEnabled,
		ManifestPublisherMode:   stringValue(config.ManifestRegistry, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Mode }),
		Downstream: model.DownstreamConfig{
			PlatformOrchestratorBaseURL: stringValue(config.Downstream, func(v *model.DownstreamConfig) string { return v.PlatformOrchestratorBaseURL }),
			MetaServiceBaseURL:          stringValue(config.Downstream, func(v *model.DownstreamConfig) string { return v.MetaServiceBaseURL }),
			NetworkServiceBaseURL:       stringValue(config.Downstream, func(v *model.DownstreamConfig) string { return v.NetworkServiceBaseURL }),
			ConfigServiceBaseURL:        stringValue(config.Downstream, func(v *model.DownstreamConfig) string { return v.ConfigServiceBaseURL }),
		},
	})
	model.InitConfigRepo(config.Repo)
	if runtime.IsIntentMode() {
		workerCfg := runtime.ReleaseIntentWorkerConfigFromModel(config.Worker)
		runtime.StartReleaseIntentWorker(runtimeCtx, workerCfg, service.ReleaseService)
	}
	return func(shutdownCtx context.Context) error {
		runtimeCancel()
		closeErr := db.Close()
		shutdownErr := shutdown(shutdownCtx)
		if shutdownErr != nil {
			return shutdownErr
		}
		return closeErr
	}, nil
}

func initObservability(ctx context.Context, logCfg *model.LogConfig, otelCfg *model.OtelConfig, pyroscopeAddr, serviceName string) (func(context.Context) error, error) {
	opts := observability.RuntimeOptions{
		LogLevel:               "",
		LogFormat:              "",
		OtelEndpoint:           "",
		OtelProtocol:           "",
		OtelService:            resolveObservabilityServiceName(otelCfg, serviceName),
		OtelResourceAttributes: "",
		PyroscopeAddr:          pyroscopeAddr,
		ServiceOverride:        serviceName,
	}
	if logCfg != nil {
		opts.LogLevel = logCfg.Level
		opts.LogFormat = logCfg.Format
	}
	if otelCfg != nil {
		opts.OtelEndpoint = otelCfg.Endpoint
		opts.OtelProtocol = otelCfg.Protocol
		opts.OtelResourceAttributes = otelCfg.ResourceAttributes
		opts.OtelSampleRatio = otelCfg.SampleRatio
	}
	return observability.Init(ctx, opts)
}

func resolveObservabilityServiceName(otelCfg *model.OtelConfig, override string) string {
	if override != "" {
		return override
	}
	if otelCfg != nil && otelCfg.ServiceName != "" {
		return otelCfg.ServiceName
	}
	return "devflow"
}

func LoadKubeConfig() (*rest.Config, error) {
	// 1️⃣ 尝试本地 kubeconfig
	if cfg, err := loadLocalKubeConfig(); err == nil {
		cfg.WrapTransport = wrapK8sTransport()
		return cfg, nil
	}

	// 2️⃣ 回退到 InCluster
	if cfg, err := rest.InClusterConfig(); err == nil {
		cfg.WrapTransport = wrapK8sTransport()
		return cfg, nil
	}

	return nil, fmt.Errorf("failed to load kubeconfig (local & in-cluster)")
}

// loadLocalKubeConfig 从 $HOME/.kube/config 加载
func loadLocalKubeConfig() (*rest.Config, error) {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // Windows fallback
	}

	kubeconfig := filepath.Join(home, ".kube", "config")

	// 如果文件不存在，直接返回 error
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return nil, err
	}

	// 使用 kubeconfig 构建 config
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func wrapK8sTransport() func(http.RoundTripper) http.RoundTripper {
	return func(rt http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(
			rt,
			otelhttp.WithTracerProvider(otel.GetTracerProvider()),
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				// 更清晰的 span 名称
				return fmt.Sprintf("k8s.api %s %s", r.Method, r.URL.Path)
			}),
			otelhttp.WithFilter(func(r *http.Request) bool {
				if r.Method == http.MethodPost &&
					strings.HasSuffix(r.URL.Path, "/pipelineruns") {
					return false
				}
				return true
			}),
		)
	}
}

func stringValue[T any](value *T, getter func(*T) string) string {
	if value == nil {
		return ""
	}
	return getter(value)
}

func intValue[T any](value *T, getter func(*T) int) int {
	if value == nil {
		return 0
	}
	return getter(value)
}
