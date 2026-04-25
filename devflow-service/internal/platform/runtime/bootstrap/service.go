package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.uber.org/zap"
)

type Runner interface {
	Run(...string) error
}

type Options[C any, R any, E ~string] struct {
	Name               string
	RouteOptions       R
	ExecutionMode      E
	Load               func() (*C, error)
	InitRuntime        func(context.Context, *C, string) (func(context.Context) error, error)
	NewRouter          func(R) Runner
	ResolveConfigPort  func(*C) int
	SetExecutionMode   func(E)
	StartMetricsServer func(string)
	StartPprofServer   func(string)
	PortEnv            string
	DefaultPort        int
	MetricsPortEnv     string
	DefaultMetrics     int
	PprofPortEnv       string
	DefaultPprof       int
}

func Run[C any, R any, E ~string](opts Options[C, R, E]) error {
	cfg, err := opts.Load()
	if err != nil {
		return err
	}

	shutdown, err := opts.InitRuntime(context.Background(), cfg, opts.Name)
	if err != nil {
		return err
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	if opts.SetExecutionMode != nil && opts.ExecutionMode != "" {
		opts.SetExecutionMode(opts.ExecutionMode)
	}

	metricsPort := resolvePort(opts.DefaultMetrics, opts.MetricsPortEnv)
	if metricsPort > 0 && opts.StartMetricsServer != nil {
		opts.StartMetricsServer(fmt.Sprintf(":%d", metricsPort))
	}

	pprofPort := resolvePort(opts.DefaultPprof, opts.PprofPortEnv)
	if pprofPort > 0 && opts.StartPprofServer != nil {
		opts.StartPprofServer(fmt.Sprintf(":%d", pprofPort))
	}

	r := opts.NewRouter(opts.RouteOptions)
	port := resolveConfiguredPort(cfg, opts.DefaultPort, opts.PortEnv, opts.ResolveConfigPort)

	logger.Logger.Info("starting service",
		zap.String("service", opts.Name),
		zap.String("result", "starting"),
		zap.Int("listen_port", port),
		zap.Int("metrics_listen_port", metricsPort),
		zap.Int("pprof_listen_port", pprofPort),
	)

	return r.Run(fmt.Sprintf(":%d", port))
}

func resolveConfiguredPort[C any](cfg *C, defaultPort int, envKey string, resolver func(*C) int) int {
	port := defaultPort
	if cfg != nil && resolver != nil {
		if resolved := resolver(cfg); resolved > 0 {
			port = resolved
		}
	}
	if override := resolvePort(0, envKey); override > 0 {
		port = override
	}
	return port
}

func resolvePort(defaultPort int, envKey string) int {
	if envKey == "" {
		return defaultPort
	}
	if value := os.Getenv(envKey); value != "" {
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 {
			return port
		}
	}
	return defaultPort
}
