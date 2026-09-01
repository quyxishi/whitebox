package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	v1 "github.com/quyxishi/whitebox/internal/api/v1"
	"github.com/quyxishi/whitebox/internal/api/v1/probe"
	mlog "github.com/quyxishi/whitebox/internal/log"
	"github.com/quyxishi/whitebox/internal/metrics"
)

func (srv *Server) RegisterRoutes() http.Handler {
	r := gin.New()

	if mlog.Enabled(slog.LevelDebug) {
		r.Use(gin.Logger())
	}

	r.Use(v1.GlobalErrorHandler())

	// Configure CORS at the root level
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	probe.RegisterRoutes(&r.RouterGroup, probe.NewProbeHandler(srv.configWrapper, srv.pool))
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	r.NoRoute(v1.NotFoundHandler())
	pprof.Register(r)

	return r
}
