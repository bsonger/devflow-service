package http

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
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
			httpx.WriteUnauthorized(c)
			c.Abort()
			return
		}
		c.Next()
	}
}

func resolveObserverToken(explicit string) string {
	if token := strings.TrimSpace(explicit); token != "" {
		return token
	}
	if token := strings.TrimSpace(ObserverSharedToken); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("RUNTIME_OBSERVER_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("DEVFLOW_OBSERVER_TOKEN"))
}
