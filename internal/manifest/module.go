package manifest

import (
	manifesthttp "github.com/bsonger/devflow-service/internal/manifest/transport/http"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *manifesthttp.ManifestHandler
}

func NewModule() Module {
	return Module{
		handler: manifesthttp.NewManifestHandler(manifestservice.ManifestService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
