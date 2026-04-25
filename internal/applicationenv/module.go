package applicationenv

import (
	appconfighttp "github.com/bsonger/devflow-service/internal/appconfig/service"
	"github.com/bsonger/devflow-service/internal/application/service"
	"github.com/bsonger/devflow-service/internal/applicationenv/repository"
	applicationenvservice "github.com/bsonger/devflow-service/internal/applicationenv/service"
	applicationenvhttp "github.com/bsonger/devflow-service/internal/applicationenv/transport/http"
	envservice "github.com/bsonger/devflow-service/internal/environment/service"
	workloadconfigservice "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *applicationenvhttp.Handler
}

func NewModule() Module {
	return Module{
		handler: applicationenvhttp.NewHandler(applicationenvservice.NewService(
			repository.NewPostgresStore(),
			service.DefaultService,
			envservice.DefaultService,
			appconfighttp.NewAppConfigService(nil),
			workloadconfigservice.NewWorkloadConfigService(),
		)),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
