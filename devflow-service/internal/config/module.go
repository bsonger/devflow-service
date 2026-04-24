package config

import (
	confighttp "github.com/bsonger/devflow-service/internal/config/transport/http"
	configservice "github.com/bsonger/devflow-service/internal/config/service"
	platformconfigrepo "github.com/bsonger/devflow-service/internal/platform/configrepo"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *confighttp.Handler
}

func NewModule() Module {
	appConfigSvc := configservice.NewAppConfigService(platformconfigrepo.DefaultRepository)
	workloadConfigSvc := configservice.NewWorkloadConfigService()

	return Module{
		handler: confighttp.NewHandler(appConfigSvc, workloadConfigSvc),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
