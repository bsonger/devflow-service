package observability

import (
	"sync"
	"testing"
)

func TestOperationLoggerName(t *testing.T) {
	tests := map[string]string{
		"":                  "business.event",
		"application":       "business.event",
		"release_service":   "release.lifecycle",
		"runtime_service":   "runtime.state",
		"dependency_client": "dependency.client",
		"worker":            "worker.lifecycle",
		"service":           "service.lifecycle",
		"db":                "db.query",
	}
	for input, want := range tests {
		if got := operationLoggerName(input); got != want {
			t.Fatalf("operationLoggerName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestInitRuntimeReleaseMetrics(t *testing.T) {
	resetRuntimeMetricsForTest()
	initRuntimeReleaseMetrics()
	if runtimeMetricsInitErr != nil {
		t.Fatalf("initRuntimeReleaseMetrics error = %v", runtimeMetricsInitErr)
	}
	if runtimeReleaseReconcileTotal == nil {
		t.Fatal("runtimeReleaseReconcileTotal is nil")
	}
	if runtimeReleaseWritebackTotal == nil {
		t.Fatal("runtimeReleaseWritebackTotal is nil")
	}
	if runtimeTerminalLabelUpdateTotal == nil {
		t.Fatal("runtimeTerminalLabelUpdateTotal is nil")
	}
	if runtimeObservedWorkloadStateTotal == nil {
		t.Fatal("runtimeObservedWorkloadStateTotal is nil")
	}
}

func resetRuntimeMetricsForTest() {
	runtimeMetricsOnce = sync.Once{}
	runtimeMetricsInitErr = nil
	runtimeReleaseReconcileTotal = nil
	runtimeReleaseWritebackTotal = nil
	runtimeTerminalLabelUpdateTotal = nil
	runtimeObservedWorkloadStateTotal = nil
}
