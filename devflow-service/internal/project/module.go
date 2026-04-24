package project

import (
	projectapp "github.com/bsonger/devflow-service/internal/project/application"
	projecthttp "github.com/bsonger/devflow-service/internal/project/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *projecthttp.Handler
}

func NewModule() Module {
	return Module{
		handler: projecthttp.NewHandler(projectapp.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
