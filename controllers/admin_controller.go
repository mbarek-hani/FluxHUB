package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

// AdminController gère les endpoints d'administration
type AdminController struct {
	gitManager *services.GitManager
	signer     *services.Signer
	packager   *services.Packager
	scanner    *services.CodeScanner
}

// NewAdminController crée une nouvelle instance du contrôleur admin
func NewAdminController(
	gm *services.GitManager,
	sg *services.Signer,
	pk *services.Packager,
	sc *services.CodeScanner,
) *AdminController {
	return &AdminController{
		gitManager: gm,
		signer:     sg,
		packager:   pk,
		scanner:    sc,
	}
}

// ReviewResponse représente la réponse complète pour la revue d'un plugin
type ReviewResponse struct {
	Plugin     models.Plugin     `json:"plugin"`
	Versions   []models.Version  `json:"versions"`
	ScanReport models.ScanReport `json:"scan_report"`
	Diff       string            `json:"diff,omitempty"`
}

// Review - GET /v1/admin/review/:id
// Récupère les métadonnées et le diff pour la revue humaine
func (ac *AdminController) Review(c *gin.Context) {
	id := c.Param("id")

	// Paramètres optionnels pour le diff
	fromRef := c.Query("from")
	toRef := c.Query("to")

	// Récupérer le plugin
	var plugin models.Plugin
	if result := database.DB.Preload("Versions").First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	// Désérialiser le rapport de scan
	var scanReport models.ScanReport
	if plugin.ScanResult != "" {
		if err := json.Unmarshal([]byte(plugin.ScanResult), &scanReport); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Impossible de désérialiser le rapport de scan",
			})
			return
		}
	}

	response := ReviewResponse{
		Plugin:     plugin,
		Versions:   plugin.Versions,
		ScanReport: scanReport,
	}

	// Générer le diff si des références sont fournies
	if fromRef != "" && toRef != "" {
		diff, err := ac.gitManager.GenerateDiff(plugin.ID, fromRef, toRef)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Impossible de générer le diff",
				"details": err.Error(),
			})
			return
		}
		response.Diff = diff
	}

	c.JSON(http.StatusOK, response)
}

// GetDiff - GET /v1/admin/diff/:id
// Retourne uniquement le diff en texte brut (compatible Diff2Html)
func (ac *AdminController) GetDiff(c *gin.Context) {
	id := c.Param("id")
	fromRef := c.Query("from")
	toRef := c.Query("to")

	if fromRef == "" || toRef == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Les paramètres 'from' et 'to' sont requis",
		})
		return
	}

	// Vérifier que le plugin existe
	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	diff, err := ac.gitManager.GenerateDiff(plugin.ID, fromRef, toRef)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Impossible de générer le diff",
			"details": err.Error(),
		})
		return
	}

	// Retourner le diff en texte brut (format unified diff, compatible Diff2Html)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, diff)
}

// ApproveRequest représente la payload d'approbation
type ApproveRequest struct {
	Version string `json:"version" binding:"required"`
	Comment string `json:"comment"`
	AdminID string `json:"user_id" binding:"required"`
}

// Approve - POST /v1/admin/approve/:id
// Approuve un plugin, génère le ZIP et la signature
func (ac *AdminController) Approve(c *gin.Context) {
	id := c.Param("id")

	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Données invalides",
			"details": err.Error(),
		})
		return
	}

	// Récupérer le plugin
	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	// Vérifier que le plugin n'est pas déjà approuvé/rejeté
	if plugin.Status == models.StatusApproved {
		c.JSON(http.StatusConflict, gin.H{"error": "Plugin déjà approuvé"})
		return
	}

	// Vérifier que la version existe
	var version models.Version
	if result := database.DB.
		Where("plugin_id = ? AND tag = ?", plugin.ID, req.Version).
		First(&version); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Version/tag non trouvé. Assurez-vous que le tag existe dans le dépôt.",
		})
		return
	}

	// Checkout du tag à packager
	if err := ac.gitManager.CheckoutTag(plugin.ID, req.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Impossible de checkout le tag",
			"details": err.Error(),
		})
		return
	}

	// Créer le ZIP
	repoPath := ac.gitManager.GetRepoPathForPlugin(plugin.ID)
	zipPath, err := ac.packager.PackagePlugin(repoPath, plugin.ID, req.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Impossible de créer l'archive ZIP",
			"details": err.Error(),
		})
		return
	}

	// Signer le ZIP
	signature, sha256Hash, err := ac.signer.SignFile(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Impossible de signer le fichier ZIP",
			"details": err.Error(),
		})
		return
	}

	// Mettre à jour la version avec la signature
	if err := database.DB.Model(&version).Updates(map[string]interface{}{
		"signature":   signature,
		"sha256_hash": sha256Hash,
		"zip_path":    zipPath,
		"changelog":   req.Comment,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Impossible de mettre à jour la version",
		})
		return
	}

	// Mettre à jour le statut du plugin
	if err := database.DB.Model(&plugin).Updates(map[string]interface{}{
		"status":          models.StatusApproved,
		"current_version": req.Version,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Impossible de mettre à jour le statut du plugin",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Plugin approuvé avec succès",
		"plugin_id":   plugin.ID,
		"version":     req.Version,
		"sha256_hash": sha256Hash,
		"signature":   signature,
	})
}

// Reject - POST /v1/admin/reject/:id
// Rejette un plugin avec une raison
func (ac *AdminController) Reject(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Reason  string `json:"reason" binding:"required"`
		AdminID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	database.DB.Model(&plugin).Update("status", models.StatusRejected)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Plugin rejeté",
		"plugin_id": plugin.ID,
		"reason":    req.Reason,
	})
}

// ListPending - GET /v1/admin/plugins/pending
// Liste les plugins en attente de revue
func (ac *AdminController) ListPending(c *gin.Context) {
	var plugins []models.Plugin

	database.DB.
		Where("status = ?", models.StatusPending).
		Preload("Versions").
		Order("created_at ASC").
		Find(&plugins)

	c.JSON(http.StatusOK, gin.H{
		"data":  plugins,
		"count": len(plugins),
	})
}

// RescanPlugin - POST /v1/admin/rescan/:id
// Relance l'analyse statique sur un plugin
func (ac *AdminController) RescanPlugin(c *gin.Context) {
	id := c.Param("id")

	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	repoPath := ac.gitManager.GetRepoPathForPlugin(plugin.ID)
	scanReport, err := ac.scanner.ScanDirectory(repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erreur lors du re-scan",
			"details": err.Error(),
		})
		return
	}

	scanJSON, _ := ac.scanner.ScanReportToJSON(scanReport)
	database.DB.Model(&plugin).Update("scan_result", scanJSON)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Re-scan effectué",
		"scan_report": scanReport,
	})
}
