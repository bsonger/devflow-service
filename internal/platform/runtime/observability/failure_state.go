package observability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
)

type FailureSnapshot struct {
	FailedAt  string `json:"failed_at"`
	Component string `json:"component"`
	Operation string `json:"operation,omitempty"`
	Target    string `json:"target,omitempty"`
	Result    string `json:"result"`
	Message   string `json:"message"`
}

var (
	lastFailureMu sync.RWMutex
	lastFailure   *FailureSnapshot
)

func RecordFailure(snapshot FailureSnapshot) {
	snapshot = normalizeFailureSnapshot(snapshot)

	lastFailureMu.Lock()
	copy := snapshot
	lastFailure = &copy
	lastFailureMu.Unlock()

	persistFailureSnapshot(snapshot)
}

func LastFailure() *FailureSnapshot {
	lastFailureMu.RLock()
	if lastFailure != nil {
		copy := *lastFailure
		lastFailureMu.RUnlock()
		return &copy
	}
	lastFailureMu.RUnlock()

	snapshot, err := loadFailureSnapshot()
	if err != nil || snapshot == nil {
		return nil
	}

	lastFailureMu.Lock()
	lastFailure = snapshot
	copy := *snapshot
	lastFailureMu.Unlock()
	return &copy
}

func normalizeFailureSnapshot(snapshot FailureSnapshot) FailureSnapshot {
	snapshot.Component = strings.TrimSpace(snapshot.Component)
	snapshot.Operation = strings.TrimSpace(snapshot.Operation)
	snapshot.Target = strings.TrimSpace(snapshot.Target)
	snapshot.Message = strings.TrimSpace(snapshot.Message)
	snapshot.Result = strings.TrimSpace(snapshot.Result)
	if snapshot.Result == "" {
		snapshot.Result = "error"
	}
	if snapshot.Message == "" {
		snapshot.Message = "runtime failure"
	}
	if snapshot.FailedAt == "" {
		snapshot.FailedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return snapshot
}

func persistFailureSnapshot(snapshot FailureSnapshot) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}

	path := failureSnapshotPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tempPath, path)
}

func loadFailureSnapshot() (*FailureSnapshot, error) {
	data, err := os.ReadFile(failureSnapshotPath())
	if err != nil {
		return nil, err
	}

	var snapshot FailureSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	normalized := normalizeFailureSnapshot(snapshot)
	return &normalized, nil
}

func failureSnapshotPath() string {
	service := sanitizeFileComponent(logger.ServiceName())
	if service == "" {
		service = "devflow"
	}
	return filepath.Join(os.TempDir(), service+"-last-failure.json")
}

func sanitizeFileComponent(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	return out.String()
}
