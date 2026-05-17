package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

// PluginController gère les endpoints publics des plugins
type PluginController struct {
	gitManager *services.GitManager
	scanner    *services.CodeScanner
}

func NewPluginController(gm *services.GitManager, sc *services.CodeScanner) *PluginController {
	return &PluginController{
		gitManager: gm,
		scanner:    sc,
	}
}

// ListApproved - GET /v1/plugins
// Retourne la liste publique des plugins approuvés
func (pc *PluginController) ListApproved(c *gin.Context) {
	var plugins []models.Plugin

	// Pagination
	page := 1
	pageSize := 20

	result := database.DB.
		Where("status = ?", models.StatusApproved).
		Preload("Versions").
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&plugins)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Impossible de récupérer les plugins",
		})
		return
	}

	// Compter le total
	var total int64
	database.DB.Model(&models.Plugin{}).Where("status = ?", models.StatusApproved).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"data": plugins,
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetScanResult retourne le résultat d'analyse d'un plugin (pour debug)
func (pc *PluginController) GetScanResult(c *gin.Context) {
	id := c.Param("id")

	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	// Désérialiser le JSON du scan
	var scanReport models.ScanReport
	if plugin.ScanResult != "" {
		json.Unmarshal([]byte(plugin.ScanResult), &scanReport)
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin_id":   plugin.ID,
		"plugin_name": plugin.Name,
		"status":      plugin.Status,
		"scan_report": scanReport,
	})
}
