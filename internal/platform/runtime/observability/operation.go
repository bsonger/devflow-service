package observability

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"go.uber.org/zap"
)

func OperationLogger(ctx context.Context, component, operation, resource string, fields ...zap.Field) *zap.Logger {
	log := logger.NamedLoggerFromContext(ctx, operationLoggerName(component))
	if log == nil {
		log = zap.NewNop()
	}
	base := make([]zap.Field, 0, len(fields)+2)
	if strings.TrimSpace(operation) != "" {
		base = append(base, zap.String("operation", strings.TrimSpace(operation)))
	}
	if strings.TrimSpace(resource) != "" {
		base = append(base, zap.String("resource", strings.TrimSpace(resource)))
	}
	base = append(base, fields...)
	return log.With(base...)
}

func LogOperationSuccess(log *zap.Logger, msg string, fields ...zap.Field) {
	if log == nil {
		log = zap.NewNop()
	}
	fields = append([]zap.Field{zap.String("result", "success")}, fields...)
	log.Info(msg, fields...)
}

func LogOperationFailure(log *zap.Logger, msg string, err error, fields ...zap.Field) {
	if log == nil {
		log = zap.NewNop()
	}
	base := []zap.Field{
		zap.String("result", "error"),
		zap.String("error_code", operationErrorCode(err)),
	}
	base = append(base, fields...)
	if err != nil {
		base = append(base, zap.Error(err))
	}
	log.Error(msg, base...)
}

func operationLoggerName(component string) string {
	switch strings.TrimSpace(component) {
	case "release_service", "release_runtime", "release_operator":
		return "release.lifecycle"
	case "runtime_service", "runtime_observer", "runtime_operator":
		return "runtime.state"
	case "dependency_client":
		return "dependency.client"
	case "worker":
		return "worker.lifecycle"
	case "service":
		return "service.lifecycle"
	case "db":
		return "db.query"
	default:
		return "business.event"
	}
}

func operationErrorCode(err error) string {
	if code := sharederrs.Code(err); code != "" {
		return code
	}
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sql.ErrNoRows):
		return sharederrs.CodeNotFound
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return sharederrs.CodeInternal
	}
}
