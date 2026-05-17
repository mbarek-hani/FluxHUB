package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
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
	scanner    *services.CodeScanner
}

func NewDeveloperController(
	sessions *services.SessionStore,
	gm *services.GitManager,
	sc *services.CodeScanner,
) *DeveloperController {
	return &DeveloperController{
		sessions:   sessions,
		gitManager: gm,
		scanner:    sc,
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
	database.DB.Model(&models.Plugin{}).Where("developer_id = ? AND status = ?", devID, "pending").Count(&pending)
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

		// Parse scan result
		scanIssues := 0
		hasCritical := false
		if p.ScanResult != "" {
			var scan models.ScanReport
			if json.Unmarshal([]byte(p.ScanResult), &scan) == nil {
				scanIssues = scan.TotalIssues
				hasCritical = scan.HasDangerousCode
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
			ScanIssues:     scanIssues,
			HasCritical:    hasCritical,
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

	name := c.PostForm("name")
	repoURL := c.PostForm("repo_url")
	description := c.PostForm("description")

	if name == "" || repoURL == "" {
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

	// Check name uniqueness
	var count int64
	database.DB.Model(&models.Plugin{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		c.Redirect(http.StatusFound, "/dev/submit?error=name_taken")
		return
	}

	plugin := models.Plugin{
		Name:        name,
		RepoURL:     repoURL,
		DeveloperID: devID,
		Description: description,
		Status:      models.StatusPending,
	}

	if err := database.DB.Create(&plugin).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/submit?error=server")
		return
	}

	// Clone and scan in background
	go dc.processPlugin(plugin.ID, repoURL)

	c.Redirect(http.StatusFound, "/dev/dashboard?submitted=1")
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

	var scanReport models.ScanReport
	if plugin.ScanResult != "" {
		json.Unmarshal([]byte(plugin.ScanResult), &scanReport)
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
		scanReport,
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

// processPlugin clones and scans in background
func (dc *DeveloperController) processPlugin(pluginID, repoURL string) {
	slog.Info(fmt.Sprintf("[Dev] Processing plugin %s", pluginID))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cloneResult, err := dc.gitManager.CloneRepository(ctx, repoURL, pluginID)
	if err != nil {
		slog.Info(fmt.Sprintf("[Dev] Clone failed for %s: %v", pluginID, err))
		database.DB.Model(&models.Plugin{}).
			Where("id = ?", pluginID).
			Update("status", models.StatusRejected)
		return
	}

	currentVersion := ""
	if len(cloneResult.Tags) > 0 {
		currentVersion = cloneResult.Tags[0]
	}

	scanReport, err := dc.scanner.ScanDirectory(cloneResult.LocalPath)
	if err != nil {
		slog.Info(fmt.Sprintf("[Dev] Scan failed for %s: %v", pluginID, err))
	}

	scanJSON := ""
	if scanReport != nil {
		b, _ := json.Marshal(scanReport)
		scanJSON = string(b)
	}

	database.DB.Model(&models.Plugin{}).
		Where("id = ?", pluginID).
		Updates(map[string]interface{}{
			"current_version": currentVersion,
			"scan_result":     scanJSON,
		})

	for _, tag := range cloneResult.Tags {
		changelog, _ := dc.gitManager.ExtractChangelog(pluginID, tag)
		version := models.Version{
			PluginID:  pluginID,
			Tag:       tag,
			Changelog: changelog,
		}
		database.DB.Where(models.Version{PluginID: pluginID, Tag: tag}).
			FirstOrCreate(&version)
	}

	slog.Info(fmt.Sprintf("[Dev] Plugin %s processed. Tags: %v", pluginID, cloneResult.Tags))
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

	var scanReport models.ScanReport
	if plugin.ScanResult != "" {
		json.Unmarshal([]byte(plugin.ScanResult), &scanReport)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              plugin.ID,
		"status":          plugin.Status,
		"current_version": plugin.CurrentVersion,
		"scan_issues":     scanReport.TotalIssues,
		"has_critical":    scanReport.HasDangerousCode,
		"versions":        len(plugin.Versions),
	})
}

// Needed for dev_submit page — pre-format avatar letter safely
func safeFirst(s, fallback string) string {
	r := []rune(s)
	if len(r) > 0 {
		return string(r[0])
	}
	return fallback
}

// GetPublicKeyForDev serves the public key
func (dc *DeveloperController) GetPublicKey(c *gin.Context) {
	c.File("./keys/nexus_public.pem")
}

// Re-export template.JS for use in templates
var _ = template.JS("")
