package network

import (
	networkhttp "github.com/bsonger/devflow-service/internal/network/transport/http"
	networkrepo "github.com/bsonger/devflow-service/internal/network/repository"
	networkservice "github.com/bsonger/devflow-service/internal/network/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *networkhttp.Handler
}

func NewModule() Module {
	store := networkrepo.NewPostgresStore()
	serviceSvc := networkservice.NewServiceService(store)
	routeSvc := networkservice.NewRouteService(serviceSvc)
	return Module{
		handler: networkhttp.NewHandler(serviceSvc, routeSvc),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
