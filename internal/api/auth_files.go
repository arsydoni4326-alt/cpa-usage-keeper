package api

import (
	"errors"
	"net/http"
	"strings"

	"cpa-usage-keeper/internal/service"
	"github.com/gin-gonic/gin"
)

type authFilesRequest struct {
	Names []string `json:"names"`
}

type authFilesStatusRequest struct {
	Names    []string `json:"names"`
	Disabled bool     `json:"disabled"`
}

func registerAuthFileManagementRoutes(router gin.IRoutes, provider service.AuthFilesManagementProvider) {
	router.PATCH("/auth-files/status", func(c *gin.Context) {
		if provider == nil {
			writeInternalError(c, "auth files management provider is not configured", nil)
			return
		}

		var request authFilesStatusRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
			return
		}
		names, ok := cleanAuthFilesRequestNames(request.Names)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
			return
		}

		response, err := provider.SetAuthFilesDisabled(c.Request.Context(), names, request.Disabled)
		if err != nil {
			if errors.Is(err, service.ErrAuthFilesManagementValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
				return
			}
			writeInternalError(c, "auth files status update failed", err)
			return
		}
		c.JSON(http.StatusOK, response)
	})

	router.DELETE("/auth-files", func(c *gin.Context) {
		if provider == nil {
			writeInternalError(c, "auth files management provider is not configured", nil)
			return
		}

		var request authFilesRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
			return
		}
		names, ok := cleanAuthFilesRequestNames(request.Names)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
			return
		}

		response, err := provider.DeleteAuthFiles(c.Request.Context(), names)
		if err != nil {
			if errors.Is(err, service.ErrAuthFilesManagementValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
				return
			}
			writeInternalError(c, "auth files delete failed", err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
}

func cleanAuthFilesRequestNames(names []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(names))
	cleanNames := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleanNames = append(cleanNames, trimmed)
	}
	return cleanNames, len(cleanNames) > 0
}
