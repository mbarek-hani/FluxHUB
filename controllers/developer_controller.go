package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
	"github.com/mbarek-hani/FluxHUB/views/pages"
)

type DeveloperController struct {
	sessions   *services.SessionStore
	gitManager *services.GitManager
}

func NewDeveloperController(
	sessions *services.SessionStore,
	gm *services.GitManager,
) *DeveloperController {
	return &DeveloperController{
		sessions:   sessions,
		gitManager: gm,
	}
}

// ---- Auth helpers ----

func (dc *DeveloperController) getDevID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (dc *DeveloperController) getDevUsername(c *gin.Context) string {
	if v, ok := c.Get("user_username"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (dc *DeveloperController) getDev(c *gin.Context) *models.User {
	var dev models.User
	database.DB.First(&dev, "id = ?", dc.getDevID(c))
	return &dev
}

// ---- Pages ----

func (dc *DeveloperController) Dashboard(c *gin.Context) {
	devID := dc.getDevID(c)

	dev := dc.getDev(c)

	// Load plugins with versions
	var plugins []models.Plugin
	database.DB.
		Preload("Versions").
		Where("developer_id = ?", devID).
		Order("created_at DESC").
		Find(&plugins)

	// Stats
	var pending, approved, rejected int64
	database.DB.Model(&models.Plugin{}).Where("developer_id = ? AND (status = ? OR id IN (SELECT plugin_id FROM versions WHERE signature = ?))", devID, "pending", "").Count(&pending)
	database.DB.Model(&models.Plugin{}).Where("developer_id = ? AND status = ?", devID, "approved").Count(&approved)
	database.DB.Model(&models.Plugin{}).Where("developer_id = ? AND status = ?", devID, "rejected").Count(&rejected)

	// Build plugin rows with pre-formatted data
	rows := make([]pages.DevPluginRow, len(plugins))
	for i, p := range plugins {
		vRows := make([]pages.VersionRow, len(p.Versions))
		for j, v := range p.Versions {
			vRows[j] = pages.VersionRow{
				Tag:       v.Tag,
				Signed:    v.Signature != "",
				Changelog: v.Changelog,
			}
		}

		rows[i] = pages.DevPluginRow{
			ID:             p.ID,
			Name:           p.Name,
			Description:    p.Description,
			RepoURL:        p.RepoURL,
			CurrentVersion: p.CurrentVersion,
			Status:         string(p.Status),
			CreatedAt:      p.CreatedAt.Format("Jan 02, 2006"),
			UpdatedAt:      p.UpdatedAt.Format("Jan 02, 2006 15:04"),
			VersionCount:   len(p.Versions),
			Versions:       vRows,
		}
	}

	stats := pages.DevDashboardStats{
		TotalPlugins: len(plugins),
		Pending:      int(pending),
		Approved:     int(approved),
		Rejected:     int(rejected),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.DevDashboard(dev.Username, dev.FullName, dev.DevAvatarURL(), stats, rows).Render(c.Request.Context(), c.Writer)
}

func (dc *DeveloperController) ShowSubmit(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.DevSubmit(dc.getDevUsername(c), dc.getDev(c).DevAvatarURL(), c.Query("error")).Render(c.Request.Context(), c.Writer)
}

func (dc *DeveloperController) Submit(c *gin.Context) {
	devID := dc.getDevID(c)

	repoURL := c.PostForm("repo_url")

	if repoURL == "" {
		c.Redirect(http.StatusFound, "/dev/submit?error=required")
		return
	}

	// Validate repoURL format and extract owner
	repoParts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "github.com/")
	if len(repoParts) != 2 {
		c.Redirect(http.StatusFound, "/dev/submit?error=invalid_repo")
		return
	}
	
	ownerRepo := strings.Split(repoParts[1], "/")
	if len(ownerRepo) < 2 {
		c.Redirect(http.StatusFound, "/dev/submit?error=invalid_repo")
		return
	}
	
	owner := ownerRepo[0]
	if owner != dc.getDevUsername(c) {
		c.Redirect(http.StatusFound, "/dev/submit?error=not_owner")
		return
	}

	// Generate a temporary ID for cloning
	tempID := fmt.Sprintf("temp-%d", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	_, err := dc.gitManager.CloneRepository(ctx, repoURL, tempID)
	if err != nil {
		slog.Error(fmt.Sprintf("[Dev] Synchronous clone failed for %s: %v", repoURL, err))
		c.Redirect(http.StatusFound, "/dev/submit?error=clone_failed")
		return
	}

	tempPath := dc.gitManager.GetRepoPathForPlugin(tempID)
	defer func() {
		// Clean up temp folder if it still exists
		if _, err := os.Stat(tempPath); err == nil {
			os.RemoveAll(tempPath)
		}
	}()

	// Parse manifest.json at root of cloned repository
	manifestPath := filepath.Join(tempPath, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		c.Redirect(http.StatusFound, "/dev/submit?error=missing_manifest")
		return
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		slog.Error(fmt.Sprintf("[Dev] Failed to read manifest: %v", err))
		c.Redirect(http.StatusFound, "/dev/submit?error=invalid_manifest")
		return
	}

	var manifestData struct {
		Identifiant string `json:"identifiant"`
		Nom         string `json:"nom"`
		Auteur      string `json:"auteur"`
		Author      string `json:"author"`
		Description string `json:"description"`
		Licence     string `json:"licence"`
		License     string `json:"license"`
	}

	if err := json.Unmarshal(manifestBytes, &manifestData); err != nil {
		slog.Error(fmt.Sprintf("[Dev] Failed to parse manifest: %v", err))
		c.Redirect(http.StatusFound, "/dev/submit?error=invalid_manifest")
		return
	}

	identifiant := strings.TrimSpace(manifestData.Identifiant)
	if identifiant == "" {
		c.Redirect(http.StatusFound, "/dev/submit?error=missing_identifiant")
		return
	}

	// Check if this identifiant already exists in database
	var existingPlugin models.Plugin
	err = database.DB.Where("id = ?", identifiant).First(&existingPlugin).Error
	
	finalPath := dc.gitManager.GetRepoPathForPlugin(identifiant)

	if err == nil {
		// It exists!
		// Check if it belongs to the same developer
		if existingPlugin.DeveloperID != devID {
			slog.Warn(fmt.Sprintf("[Dev] User %s tried to submit plugin %s which belongs to %s", devID, identifiant, existingPlugin.DeveloperID))
			c.Redirect(http.StatusFound, "/dev/submit?error=already_exists_different_user")
			return
		}

		// Same developer: this is an update!
		// 1. Delete the existing cloned directory if it exists
		if err := os.RemoveAll(finalPath); err != nil {
			slog.Error(fmt.Sprintf("[Dev] Failed to remove existing directory %s: %v", finalPath, err))
		}

		// 2. Rename/Move the temp clone to final directory
		if err := os.Rename(tempPath, finalPath); err != nil {
			slog.Error(fmt.Sprintf("[Dev] Failed to move temp clone to final destination: %v", err))
			c.Redirect(http.StatusFound, "/dev/submit?error=server")
			return
		}

		// Extract fields from manifest
		name := identifiant
		if manifestData.Nom != "" {
			name = manifestData.Nom
		}

		author := manifestData.Auteur
		if author == "" {
			author = manifestData.Author
		}

		licence := manifestData.Licence
		if licence == "" {
			licence = manifestData.License
		}

		// 3. Update existing plugin record
		existingPlugin.Name = name
		existingPlugin.RepoURL = repoURL
		existingPlugin.Description = manifestData.Description
		existingPlugin.Author = author
		existingPlugin.Licence = licence
		if existingPlugin.Status != models.StatusApproved {
			existingPlugin.Status = models.StatusPending
		}


		if err := database.DB.Save(&existingPlugin).Error; err != nil {
			slog.Error(fmt.Sprintf("[Dev] Failed to update plugin in DB: %v", err))
			c.Redirect(http.StatusFound, "/dev/submit?error=server")
			return
		}

		// Trigger background processing (scan and version updates)
		go dc.processPlugin(identifiant)

		c.Redirect(http.StatusFound, "/dev/dashboard?submitted=1")
		return

	} else {
		// It does not exist: this is a new plugin submission!
		// 1. Rename/Move the temp clone to final directory
		if err := os.Rename(tempPath, finalPath); err != nil {
			slog.Error(fmt.Sprintf("[Dev] Failed to move temp clone to final destination: %v", err))
			c.Redirect(http.StatusFound, "/dev/submit?error=server")
			return
		}

		// Extract fields from manifest
		name := identifiant
		if manifestData.Nom != "" {
			name = manifestData.Nom
		}

		author := manifestData.Auteur
		if author == "" {
			author = manifestData.Author
		}

		licence := manifestData.Licence
		if licence == "" {
			licence = manifestData.License
		}

		// 2. Create the new plugin record
		newPlugin := models.Plugin{
			ID:          identifiant,
			Name:        name,
			RepoURL:     repoURL,
			DeveloperID: devID,
			Description: manifestData.Description,
			Author:      author,
			Licence:     licence,
			Status:      models.StatusPending,
		}

		if err := database.DB.Create(&newPlugin).Error; err != nil {
			slog.Error(fmt.Sprintf("[Dev] Failed to create plugin in DB: %v", err))
			c.Redirect(http.StatusFound, "/dev/submit?error=server")
			return
		}

		// Trigger background processing (scan and version updates)
		go dc.processPlugin(identifiant)

		c.Redirect(http.StatusFound, "/dev/dashboard?submitted=1")
		return
	}
}

func (dc *DeveloperController) PluginDetail(c *gin.Context) {
	devID := dc.getDevID(c)
	pluginID := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.
		Preload("Versions").
		Where("id = ? AND developer_id = ?", pluginID, devID).
		First(&plugin).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/dashboard")
		return
	}

	vRows := make([]pages.DevPluginDetailVersion, len(plugin.Versions))
	for i, v := range plugin.Versions {
		vRows[i] = pages.DevPluginDetailVersion{
			Tag:        v.Tag,
			Signed:     v.Signature != "",
			SHA256Hash: v.SHA256Hash,
			CreatedAt:  v.CreatedAt.Format("Jan 02, 2006"),
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.DevPluginDetail(
		dc.getDevUsername(c),
		dc.getDev(c).DevAvatarURL(),
		plugin,
		string(plugin.Status),
		plugin.CreatedAt.Format("January 02, 2006 15:04 UTC"),
		plugin.UpdatedAt.Format("January 02, 2006 15:04 UTC"),
		vRows,
		fmt.Sprintf("/v1/plugins/download/%s", plugin.ID),
	).Render(c.Request.Context(), c.Writer)
}

func (dc *DeveloperController) ShowProfile(c *gin.Context) {
	dev := dc.getDev(c)

	success := c.Query("success") == "true"
	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.DevProfile(dev.Username, dev.DevAvatarURL(), *dev, success, c.Query("error")).Render(c.Request.Context(), c.Writer)
}

func (dc *DeveloperController) UpdateProfile(c *gin.Context) {
	dev := dc.getDev(c)

	dev.FullName = c.PostForm("full_name")
	dev.Company = c.PostForm("company")
	dev.Website = c.PostForm("website")

	if err := database.DB.Save(&dev).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/profile?error=server")
		return
	}

	c.Redirect(http.StatusFound, "/dev/profile?success=1")
}

// processPlugin scans in background and extracts tags/changelogs
func (dc *DeveloperController) processPlugin(pluginID string) {
	slog.Info(fmt.Sprintf("[Dev] Processing plugin %s", pluginID))

	// Since it is already cloned at localPath, we can read the tags
	tags, err := dc.gitManager.GetTags(pluginID)
	if err != nil {
		slog.Warn(fmt.Sprintf("[Dev] Failed to get tags for %s: %v", pluginID, err))
		tags = []string{}
	}

	currentVersion := ""
	if len(tags) > 0 {
		currentVersion = tags[0]
	}

	scanJSON := ""

	var plugin models.Plugin
	database.DB.First(&plugin, "id = ?", pluginID)

	updates := map[string]interface{}{
		"scan_result": scanJSON,
	}
	if plugin.CurrentVersion == "" || plugin.Status != models.StatusApproved {
		updates["current_version"] = currentVersion
	}

	database.DB.Model(&models.Plugin{}).
		Where("id = ?", pluginID).
		Updates(updates)


	for _, tag := range tags {
		changelog, _ := dc.gitManager.ExtractChangelog(pluginID, tag)
		version := models.Version{
			PluginID:  pluginID,
			Tag:       tag,
			Changelog: changelog,
		}
		database.DB.Where(models.Version{PluginID: pluginID, Tag: tag}).
			FirstOrCreate(&version)
	}

	slog.Info(fmt.Sprintf("[Dev] Plugin %s processed. Tags: %v", pluginID, tags))
}

// APIGetPluginStatus — AJAX polling endpoint for real-time status
func (dc *DeveloperController) APIGetPluginStatus(c *gin.Context) {
	devID := dc.getDevID(c)
	pluginID := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.
		Preload("Versions").
		Where("id = ? AND developer_id = ?", pluginID, devID).
		First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              plugin.ID,
		"status":          plugin.Status,
		"current_version": plugin.CurrentVersion,
		"scan_issues":     0,
		"has_critical":    false,
		"versions":        len(plugin.Versions),
	})
}

