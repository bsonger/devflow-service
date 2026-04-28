package release

import (
	"github.com/bsonger/devflow-service/internal/intent"
	"github.com/bsonger/devflow-service/internal/manifest"
	"github.com/gin-gonic/gin"
)

type Module struct{}

func NewModule() Module {
	return Module{}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	manifest.NewModule().RegisterRoutes(rg)
	intent.NewModule().RegisterRoutes(rg)
}
