package appservice

import (
	appservice "github.com/bsonger/devflow-service/internal/appservice/service"
	appservicehttp "github.com/bsonger/devflow-service/internal/appservice/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *appservicehttp.Handler
}

func NewModule() Module {
	return Module{
		handler: appservicehttp.NewHandler(appservice.DefaultServiceService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
