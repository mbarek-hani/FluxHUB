package controllers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

// DownloadController gère le téléchargement des plugins
type DownloadController struct {
	packager *services.Packager
	signer   *services.Signer
}

// NewDownloadController crée une nouvelle instance du contrôleur de téléchargement
func NewDownloadController(pk *services.Packager, sg *services.Signer) *DownloadController {
	return &DownloadController{
		packager: pk,
		signer:   sg,
	}
}

// Download - GET /v1/plugins/download/:id/:version
// Sert le fichier ZIP avec la signature en headers HTTP
func (dc *DownloadController) Download(c *gin.Context) {
	pluginID := c.Param("id")
	versionTag := c.Param("version")

	// Récupérer le plugin
	var plugin models.Plugin
	if result := database.DB.First(&plugin, "id = ?", pluginID); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	// Vérifier que le plugin est approuvé (SÉCURITÉ CRITIQUE)
	if !plugin.IsApproved() {
		c.JSON(http.StatusForbidden, gin.H{
			"error":  "Ce plugin n'est pas approuvé pour le téléchargement",
			"status": plugin.Status,
		})
		return
	}

	// Récupérer la version demandée
	var version models.Version
	if result := database.DB.
		Where("plugin_id = ? AND tag = ?", pluginID, versionTag).
		First(&version); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Version non trouvée",
			"version": versionTag,
		})
		return
	}

	// Vérifier que la version a une signature (= a été approuvée et packagée)
	if version.Signature == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Le package pour cette version n'est pas encore prêt",
		})
		return
	}

	// Obtenir le chemin du ZIP
	zipPath := dc.packager.GetZipPath(pluginID, versionTag)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Fichier ZIP introuvable. Veuillez contacter l'administrateur.",
		})
		return
	}

	// Nom du fichier pour le téléchargement
	fileName := fmt.Sprintf("%s-%s.zip", plugin.Name, versionTag)

	// Ajouter les headers de sécurité et de métadonnées
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", "application/zip")
	c.Header("X-Plugin-ID", plugin.ID)
	c.Header("X-Plugin-Name", plugin.Name)
	c.Header("X-Plugin-Version", versionTag)
	c.Header("X-Plugin-Signature", version.Signature)
	c.Header("X-Plugin-SHA256", version.SHA256Hash)
	c.Header("X-Plugin-Developer", plugin.DeveloperID)

	// Servir le fichier ZIP
	c.File(zipPath)
}

// GetPublicKey - GET /v1/public-key
// Retourne la clé publique RSA pour la vérification des signatures
func (dc *DownloadController) GetPublicKey(c *gin.Context) {
	publicKeyPEM, err := dc.signer.GetPublicKeyPEM()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Impossible d'exporter la clé publique",
		})
		return
	}

	c.Header("Content-Type", "application/x-pem-file")
	c.String(http.StatusOK, publicKeyPEM)
}

// GetVersionInfo - GET /v1/plugins/:id/versions
// Liste toutes les versions disponibles d'un plugin approuvé
func (dc *DownloadController) GetVersionInfo(c *gin.Context) {
	pluginID := c.Param("id")

	var plugin models.Plugin
	if result := database.DB.
		Preload("Versions").
		First(&plugin, "id = ?", pluginID); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin non trouvé"})
		return
	}

	if !plugin.IsApproved() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Plugin non approuvé"})
		return
	}

	// Construire les liens de téléchargement pour chaque version
	type VersionInfo struct {
		models.Version
		DownloadURL string `json:"download_url"`
	}

	var versionsInfo []VersionInfo
	for _, v := range plugin.Versions {
		if v.Signature != "" { // Seulement les versions packagées
			downloadURL := fmt.Sprintf("/v1/plugins/download/%s/%s", pluginID, v.Tag)
			versionsInfo = append(versionsInfo, VersionInfo{
				Version:     v,
				DownloadURL: downloadURL,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin":   plugin,
		"versions": versionsInfo,
	})
}
