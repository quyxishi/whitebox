package api

import (
	"fmt"
	"net/http"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/quyxishi/whitebox/internal/config"
)

type Server struct {
	port          int
	configWrapper *config.WhiteboxConfigWrapper
}

func NewServer(wrapper *config.WhiteboxConfigWrapper) *http.Server {
	inner := &Server{
		port:          9116,
		configWrapper: wrapper,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", inner.port),
		Handler:      inner.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
