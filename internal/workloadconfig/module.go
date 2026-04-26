package workloadconfig

import (
	workloadconfig "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	workloadconfighttp "github.com/bsonger/devflow-service/internal/workloadconfig/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *workloadconfighttp.Handler
}

func NewModule() Module {
	return Module{
		handler: workloadconfighttp.NewHandler(workloadconfig.NewWorkloadConfigService()),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
