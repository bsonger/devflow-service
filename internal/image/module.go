package image

import (
	imagehttp "github.com/bsonger/devflow-service/internal/image/transport/http"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *imagehttp.ImageHandler
}

func NewModule() Module {
	return Module{
		handler: imagehttp.NewImageHandler(imageservice.ImageService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
