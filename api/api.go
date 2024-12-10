package api

import (
	"net/http"
	"test/ContainMesh/config"
	"test/ContainMesh/docker_functions"

	"github.com/gin-gonic/gin"
)

func Start(config *config.Config) {
	r := gin.Default()
	r.GET("/api/graph", func(c *gin.Context) {
		c.JSON(http.StatusOK, docker_functions.GetGraphEncoding(config))
	})
	r.Run(":8080") // Ascolta sulla porta 8080
}
