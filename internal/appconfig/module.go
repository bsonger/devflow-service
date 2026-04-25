package appconfig

import (
	appconfighttp "github.com/bsonger/devflow-service/internal/appconfig/transport/http"
	appconfig "github.com/bsonger/devflow-service/internal/appconfig/service"
	platformconfigrepo "github.com/bsonger/devflow-service/internal/platform/configrepo"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *appconfighttp.Handler
}

func NewModule() Module {
	return Module{
		handler: appconfighttp.NewHandler(appconfig.NewAppConfigService(platformconfigrepo.DefaultRepository)),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
