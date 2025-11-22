package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	v1 "github.com/quyxishi/whitebox/internal/api/v1"
	"github.com/quyxishi/whitebox/internal/api/v1/probe"
)

func (srv *Server) RegisterRoutes() http.Handler {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(v1.GlobalErrorHandler())

	// Configure CORS at the root level
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	probe.RegisterRoutes(&r.RouterGroup, probe.NewProbeHandler())
	r.NoRoute(v1.NotFoundHandler())

	return r
}
