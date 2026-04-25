package intent

import (
	intenthttp "github.com/bsonger/devflow-service/internal/intent/transport/http"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *intenthttp.IntentHandler
}

func NewModule() Module {
	return Module{
		handler: intenthttp.NewIntentHandler(intentservice.IntentService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
