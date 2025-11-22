package api

import (
	"fmt"
	"net/http"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port int
}

func NewServer() *http.Server {
	inner := &Server{
		port: 9116,
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
