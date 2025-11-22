package probe

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, handler *ProbeHandler) {
	rg.Use(NoCacheMiddleware())
	rg.GET("/probe", handler.Probe)
}
