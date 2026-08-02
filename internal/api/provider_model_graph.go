package api

import (
	"net/http"

	"cpa-usage-keeper/internal/service"
	"github.com/gin-gonic/gin"
)

func registerProviderModelGraphRoutes(router gin.IRoutes, provider service.ProviderModelGraphProvider) {
	router.GET("/provider-model-graph", func(c *gin.Context) {
		if provider == nil {
			writeInternalError(c, "provider model graph provider is not configured", nil)
			return
		}
		response, err := provider.GetProviderModelGraph(c.Request.Context())
		if err != nil {
			writeInternalError(c, "provider model graph fetch failed", err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
}
