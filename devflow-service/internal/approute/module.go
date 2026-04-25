package approute

import (
	approutehttp "github.com/bsonger/devflow-service/internal/approute/transport/http"
	approute "github.com/bsonger/devflow-service/internal/approute/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *approutehttp.Handler
}

func NewModule() Module {
	return Module{
		handler: approutehttp.NewHandler(approute.DefaultRouteService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
