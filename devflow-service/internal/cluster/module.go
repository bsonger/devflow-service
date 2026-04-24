package cluster

import (
	clustersvc "github.com/bsonger/devflow-service/internal/cluster/application"
	clusterhttp "github.com/bsonger/devflow-service/internal/cluster/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *clusterhttp.Handler
}

func NewModule() Module {
	return Module{
		handler: clusterhttp.NewHandler(clustersvc.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
