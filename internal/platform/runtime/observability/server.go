package observability

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func StartMetricsServer(addr string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go serve("metrics", addr, mux)
}

func StartPprofServer(addr string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	go serve("pprof", addr, mux)
}

func serve(kind, addr string, handler http.Handler) {
	logger.Logger.Info("starting observability server",
		zap.String("kind", kind),
		zap.String("addr", addr),
	)

	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Logger.Error("observability server exited",
			zap.String("kind", kind),
			zap.String("addr", addr),
			zap.Error(err),
		)
	}
}
