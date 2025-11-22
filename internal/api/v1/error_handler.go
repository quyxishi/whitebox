package v1

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func GlobalErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[ERROR] %v\n%s\n", err, string(debug.Stack()))
				c.String(http.StatusInternalServerError, "Unexpected error: %v", err)
				c.Abort()
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			statusCode := c.Writer.Status()
			if statusCode == http.StatusOK {
				statusCode = http.StatusInternalServerError
			}

			c.String(statusCode, "Unexpected error: %s", err.Error())
		}
	}
}

func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(http.StatusNotFound, "No resource: %s '%s'", c.Request.Method, c.Request.URL.Path)
	}
}
