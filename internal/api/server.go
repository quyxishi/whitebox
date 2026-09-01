package api

import (
	"net/http"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/quyxishi/whitebox/internal/config"
)

const DefaultListenAddress = ":9116"

type Server struct {
	listenAddress string
	configWrapper *config.WhiteboxConfigWrapper
}

func NormalizeListenAddress(listenAddress string) string {
	listenAddress = strings.TrimSpace(listenAddress)

	if listenAddress == "" {
		return DefaultListenAddress
	}

	if !strings.Contains(listenAddress, ":") {
		return ":" + listenAddress
	}

	return listenAddress
}

func NewServer(wrapper *config.WhiteboxConfigWrapper, listenAddress string) *http.Server {
	inner := &Server{
		listenAddress: NormalizeListenAddress(listenAddress),
		configWrapper: wrapper,
	}

	server := &http.Server{
		Addr:         inner.listenAddress,
		Handler:      inner.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
