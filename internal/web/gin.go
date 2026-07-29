// Package web provides HTTP server utilities using gin-gonic.
package web

import (
	"github.com/gin-gonic/gin"
)

// Engine returns a new gin engine with default middleware.
func Engine() *gin.Engine {
	return gin.Default()
}
