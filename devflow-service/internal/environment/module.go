package environment

import (
	envsvc "github.com/bsonger/devflow-service/internal/environment/application"
	envhttp "github.com/bsonger/devflow-service/internal/environment/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *envhttp.Handler
}

func NewModule() Module {
	return Module{
		handler: envhttp.NewHandler(envsvc.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
