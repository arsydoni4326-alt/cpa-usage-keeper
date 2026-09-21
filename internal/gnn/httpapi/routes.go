// Package gnnhttpapi exposes the Provider Model GNN graph data over HTTP.
// The response contains the sanitized providers list plus the GNN-derived
// node/edge features and embeddings.
package gnnhttpapi

import (
	"net/http"

	"cpa-usage-keeper/internal/gnn"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Provider is the GNN graph source consumed by the routes; defined here so
// the API layer never imports service internals for this feature.
type Provider = gnn.ProviderModelGNNProvider

// RegisterRoutes exposes GET /provider-model-gnn (canonical) and
// GET /provider-model-graph (kept as a backward-compatible alias).
func RegisterRoutes(router gin.IRoutes, provider Provider) {
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
	router.GET("/provider-model-gnn", handler)
	router.GET("/provider-model-graph", handler)
}

// writeInternalError mirrors the API-layer error convention without importing
// the api package (keeps the feature domain self-contained).
func writeInternalError(c *gin.Context, message string, err error) {
	if err != nil {
		logrus.WithError(err).Error(message)
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
