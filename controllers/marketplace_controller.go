package controllers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

// parseSemver parses a semver string "vMAJOR.MINOR.PATCH" or "MAJOR.MINOR.PATCH"
// into three integers. Returns (0,0,0,false) when parsing fails.
func parseSemver(tag string) (major, minor, patch int, ok bool) {
	// Strip optional leading 'v'
	s := tag
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	n, err := fmt.Sscanf(s, "%d.%d.%d", &major, &minor, &patch)
	ok = err == nil && n == 3
	return
}

// semverGreater returns true when a is strictly newer than b.
func semverGreater(a, b string) bool {
	aMaj, aMin, aPat, aOk := parseSemver(a)
	bMaj, bMin, bPat, bOk := parseSemver(b)
	if !aOk || !bOk {
		return false
	}
	if aMaj != bMaj {
		return aMaj > bMaj
	}
	if aMin != bMin {
		return aMin > bMin
	}
	return aPat > bPat
}

type MarketplaceController struct {
	packager *services.Packager
	signer   *services.Signer
}

func NewMarketplaceController(pk *services.Packager, sg *services.Signer) *MarketplaceController {
	return &MarketplaceController{packager: pk, signer: sg}
}

type pluginSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Author         string `json:"author"`
	Description    string `json:"description"`
	RepoURL        string `json:"repo_url"`
	CurrentVersion string `json:"current_version"`
	TotalDownloads int64  `json:"total_downloads"`
}

func downloadCount(pluginID string) int64 {
	var count int64
	database.DB.Model(&models.PluginDownload{}).Where("plugin_id = ?", pluginID).Count(&count)
	return count
}

func toSummary(p models.Plugin) pluginSummary {
	return pluginSummary{
		ID:             p.ID,
		Name:           p.Name,
		Author:         p.Author,
		Description:    p.Description,
		RepoURL:        p.RepoURL,
		CurrentVersion: p.CurrentVersion,
		TotalDownloads: downloadCount(p.ID),
	}
}

//Endpoints

// ListPlugins - GET /v1/marketplace/plugins
//
// Query params:
//
//	page      int    (default 1)
//	page_size int    (default 20, max 100)
//	search    string (partial match on plugin name)
func (mc *MarketplaceController) ListPlugins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := database.DB.Model(&models.Plugin{}).Where("status = ?", models.StatusApproved)
	if search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var plugins []models.Plugin
	if err := query.
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&plugins).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugins"})
		return
	}

	summaries := make([]pluginSummary, 0, len(plugins))
	for _, p := range plugins {
		summaries = append(summaries, toSummary(p))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": summaries,
		"meta": gin.H{
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GetPlugin - GET /v1/marketplace/plugins/:id
// Returns full details for a single approved plugin.
func (mc *MarketplaceController) GetPlugin(c *gin.Context) {
	id := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.First(&plugin, "id = ? AND status = ?", id, models.StatusApproved).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toSummary(plugin)})
}

// DownloadLatest - GET /v1/marketplace/plugins/:id/download
// Downloads the latest approved version of a plugin and increments the download counter.
func (mc *MarketplaceController) DownloadLatest(c *gin.Context) {
	pluginID := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.First(&plugin, "id = ? AND status = ?", pluginID, models.StatusApproved).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	if plugin.CurrentVersion == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No released version available for this plugin"})
		return
	}

	var version models.Version
	if err := database.DB.
		Where("plugin_id = ? AND tag = ?", pluginID, plugin.CurrentVersion).
		First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Latest version record not found"})
		return
	}

	if version.Signature == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Package for this version is not yet ready"})
		return
	}

	zipPath := mc.packager.GetZipPath(pluginID, plugin.CurrentVersion)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin ZIP not found — please contact the administrator"})
		return
	}

	// Record the download event (fire-and-forget; don't fail the download on DB error)
	database.DB.Create(&models.PluginDownload{
		PluginID:   pluginID,
		VersionTag: plugin.CurrentVersion,
	})

	fileName := fmt.Sprintf("%s-%s.zip", plugin.Name, plugin.CurrentVersion)

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", "application/zip")
	c.Header("X-Plugin-ID", plugin.ID)
	c.Header("X-Plugin-Name", plugin.Name)
	c.Header("X-Plugin-Version", plugin.CurrentVersion)
	c.Header("X-Plugin-Signature", version.Signature)
	c.Header("X-Plugin-SHA256", version.SHA256Hash)
	c.Header("X-Plugin-Developer", plugin.DeveloperID)

	c.File(zipPath)
}

// CheckUpdate - GET /v1/marketplace/plugins/:id/updates?installed_version=x.y.z
// Returns whether a newer version is available compared to the client's installed version.
func (mc *MarketplaceController) CheckUpdate(c *gin.Context) {
	pluginID := c.Param("id")
	installedTag := c.Query("installed_version")

	if installedTag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "installed_version query parameter is required"})
		return
	}

	var plugin models.Plugin
	if err := database.DB.First(&plugin, "id = ? AND status = ?", pluginID, models.StatusApproved).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	if plugin.CurrentVersion == "" {
		c.JSON(http.StatusOK, gin.H{"update_available": false})
		return
	}

	// Compare using semver if possible, fall back to plain string inequality
	updateAvailable := semverGreater(plugin.CurrentVersion, installedTag)
	if !updateAvailable {
		// Guard against non-semver tags: at minimum flag if the strings differ
		updateAvailable = updateAvailable || (plugin.CurrentVersion != installedTag && !semverGreater(installedTag, plugin.CurrentVersion))
	}

	resp := gin.H{
		"update_available":  updateAvailable,
		"latest_version":    plugin.CurrentVersion,
		"installed_version": installedTag,
	}
	if updateAvailable {
		resp["download_url"] = fmt.Sprintf("/v1/marketplace/plugins/%s/download", pluginID)
	}

	c.JSON(http.StatusOK, resp)
}

// GetPublicKey - GET /v1/public-key
// Returns the RSA public key PEM for signature verification.
func (mc *MarketplaceController) GetPublicKey(c *gin.Context) {
	publicKeyPEM, err := mc.signer.GetPublicKeyPEM()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export public key",
		})
		return
	}

	c.Header("Content-Type", "application/x-pem-file")
	c.String(http.StatusOK, publicKeyPEM)
}
