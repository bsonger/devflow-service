package observability

import "testing"

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
