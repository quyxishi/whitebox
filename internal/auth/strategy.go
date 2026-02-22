package auth

import "net/http"

type HttpAuthStrategy interface {
	Name() string
	Init() error
	Issue(*http.Request) error
}
