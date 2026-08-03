package api

import (
	"net/http"

	"cpa-usage-keeper/internal/service"
	"github.com/gin-gonic/gin"
)

// registerProviderModelGNNRoutes 暴露 provider ↔ model 的 GNN 图数据。
// 响应包含脱敏后的 providers 列表以及 GNN 派生的节点/边特征与嵌入。
func registerProviderModelGNNRoutes(router gin.IRoutes, provider service.ProviderModelGNNProvider) {
	handler := func(c *gin.Context) {
		if provider == nil {
			writeInternalError(c, "provider model GNN provider is not configured", nil)
			return
		}
		response, err := provider.GetProviderModelGraph(c.Request.Context())
		if err != nil {
			writeInternalError(c, "provider model GNN fetch failed", err)
			return
		}
		c.JSON(http.StatusOK, response)
	}
	// 规范 GNN 端点（文档对齐）；/provider-model-graph 作为向后兼容别名保留。
	router.GET("/provider-model-gnn", handler)
	router.GET("/provider-model-graph", handler)
}
