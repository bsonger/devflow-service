package http

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
)

const ObserverTokenHeader = "X-Devflow-Observer-Token"
const VerifyTokenHeader = "X-Devflow-Verify-Token"

var ObserverSharedToken string

func RequireObserverToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		expected = strings.TrimSpace(expected)
		if expected == "" {
			c.Next()
			return
		}
		token := strings.TrimSpace(c.GetHeader(ObserverTokenHeader))
		if token == "" {
			token = strings.TrimSpace(c.GetHeader(VerifyTokenHeader))
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			httpx.WriteError(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func normalizeStepStatus(status model.StepStatus) model.StepStatus {
	switch strings.ToLower(string(status)) {
	case "pending":
		return model.StepPending
	case "running":
		return model.StepRunning
	case "succeeded":
		return model.StepSucceeded
	case "failed":
		return model.StepFailed
	default:
		return status
	}
}
