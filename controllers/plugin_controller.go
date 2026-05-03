package controllers

import (
	"context"
	"encoding/json"
	"log"
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

// SubmitRequest représente la payload de soumission d'un plugin
type SubmitRequest struct {
	RepoURL     string `json:"repo_url" binding:"required,url"`
	DeveloperID string `json:"developer_id" binding:"required"`
	Name        string `json:"name" binding:"required,min=3,max=100"`
	Description string `json:"description"`
}

// Submit - POST /v1/plugins/submit
// Enregistre un nouveau repo et lance le clonage/scan en arrière-plan
func (pc *PluginController) Submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Données invalides",
			"details": err.Error(),
		})
		return
	}

	// Vérifier si un plugin avec ce nom existe déjà
	var existingPlugin models.Plugin
	if result := database.DB.Where("name = ?", req.Name).First(&existingPlugin); result.Error == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Un plugin avec ce nom existe déjà",
		})
		return
	}

	// Créer le plugin en base avec le statut pending
	plugin := models.Plugin{
		Name:        req.Name,
		RepoURL:     req.RepoURL,
		DeveloperID: req.DeveloperID,
		Status:      models.StatusPending,
		Description: req.Description,
	}

	if result := database.DB.Create(&plugin); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Impossible d'enregistrer le plugin",
		})
		return
	}

	// Lancer le clonage et l'analyse en arrière-plan (goroutine)
	go pc.processPluginInBackground(plugin.ID, req.RepoURL)

	c.JSON(http.StatusAccepted, gin.H{
		"message":   "Plugin soumis avec succès. Analyse en cours...",
		"plugin_id": plugin.ID,
		"status":    plugin.Status,
	})
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

// processPluginInBackground exécute le clonage et l'analyse en goroutine
func (pc *PluginController) processPluginInBackground(pluginID, repoURL string) {
	log.Printf("[Background] Démarrage du traitement du plugin %s", pluginID)

	// Timeout de 5 minutes pour le clonage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Cloner le dépôt
	cloneResult, err := pc.gitManager.CloneRepository(ctx, repoURL, pluginID)
	if err != nil {
		log.Printf("[Background] Échec du clonage pour %s: %v", pluginID, err)
		database.DB.Model(&models.Plugin{}).
			Where("id = ?", pluginID).
			Update("status", models.StatusRejected)
		return
	}

	// Mettre à jour les tags en base
	currentVersion := ""
	if len(cloneResult.Tags) > 0 {
		currentVersion = cloneResult.Tags[0] // Tag le plus récent
	}

	// Analyser le code PHP
	scanReport, err := pc.scanner.ScanDirectory(cloneResult.LocalPath)
	if err != nil {
		log.Printf("[Background] Échec du scan pour %s: %v", pluginID, err)
		return
	}

	scanJSON, _ := pc.scanner.ScanReportToJSON(scanReport)

	// Mettre à jour le plugin en base
	updates := map[string]interface{}{
		"current_version": currentVersion,
		"scan_result":     scanJSON,
	}

	if err := database.DB.Model(&models.Plugin{}).
		Where("id = ?", pluginID).
		Updates(updates).Error; err != nil {
		log.Printf("[Background] Impossible de mettre à jour le plugin %s: %v", pluginID, err)
		return
	}

	// Enregistrer les versions (tags) en base
	for _, tag := range cloneResult.Tags {
		changelog, _ := pc.gitManager.ExtractChangelog(pluginID, tag)

		version := models.Version{
			PluginID:  pluginID,
			Tag:       tag,
			Changelog: changelog,
		}

		// Ignorer les erreurs de doublon (idempotent)
		database.DB.Where(models.Version{PluginID: pluginID, Tag: tag}).
			FirstOrCreate(&version)
	}

	log.Printf("[Background] Plugin %s traité. Tags: %v, Issues: %d, Critique: %v",
		pluginID,
		cloneResult.Tags,
		scanReport.TotalIssues,
		scanReport.HasDangerousCode,
	)
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
