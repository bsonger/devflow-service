package approute

import (
	approute "github.com/bsonger/devflow-service/internal/approute/service"
	approutehttp "github.com/bsonger/devflow-service/internal/approute/transport/http"
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
