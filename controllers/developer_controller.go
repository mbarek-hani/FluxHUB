package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

type DeveloperController struct {
	sessions   *services.SessionStore
	renderer   Renderer
	gitManager *services.GitManager
	scanner    *services.CodeScanner
}

func NewDeveloperController(
	sessions *services.SessionStore,
	renderer Renderer,
	gm *services.GitManager,
	sc *services.CodeScanner,
) *DeveloperController {
	return &DeveloperController{
		sessions:   sessions,
		renderer:   renderer,
		gitManager: gm,
		scanner:    sc,
	}
}

// ---- Auth helpers ----

func (dc *DeveloperController) getDevID(c *gin.Context) string {
	if v, ok := c.Get("dev_id"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (dc *DeveloperController) getDevUsername(c *gin.Context) string {
	if v, ok := c.Get("dev_username"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (dc *DeveloperController) getDevFullName(c *gin.Context) string {
	if v, ok := c.Get("dev_fullname"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// ---- Pages ----

func (dc *DeveloperController) ShowRegister(c *gin.Context) {
	// Already logged in?
	if cookie, err := c.Cookie("dev_session"); err == nil {
		if sess, ok := dc.sessions.Get(cookie); ok && sess.Kind == services.SessionDeveloper {
			c.Redirect(http.StatusFound, "/dev/dashboard")
			return
		}
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_register", gin.H{
		"Error": c.Query("error"),
	})
}

func (dc *DeveloperController) Register(c *gin.Context) {
	username := c.PostForm("username")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")
	fullName := c.PostForm("full_name")
	company := c.PostForm("company")
	website := c.PostForm("website")

	// Validations
	if username == "" || email == "" || password == "" {
		c.Redirect(http.StatusFound, "/dev/register?error=required")
		return
	}

	if len(password) < 8 {
		c.Redirect(http.StatusFound, "/dev/register?error=password_short")
		return
	}

	if password != confirm {
		c.Redirect(http.StatusFound, "/dev/register?error=password_mismatch")
		return
	}

	// Check uniqueness
	var existingCount int64
	database.DB.Model(&models.Developer{}).
		Where("username = ? OR email = ?", username, email).
		Count(&existingCount)

	if existingCount > 0 {
		c.Redirect(http.StatusFound, "/dev/register?error=exists")
		return
	}

	dev := models.Developer{
		Username: username,
		Email:    email,
		FullName: fullName,
		Company:  company,
		Website:  website,
	}

	if err := dev.SetPassword(password); err != nil {
		c.Redirect(http.StatusFound, "/dev/register?error=server")
		return
	}

	if err := database.DB.Create(&dev).Error; err != nil {
		slog.Info(fmt.Sprintf("Error creating developer: %v", err))
		c.Redirect(http.StatusFound, "/dev/register?error=server")
		return
	}

	// Auto-login after registration
	sessionID, err := dc.sessions.Create(
		dev.ID, dev.Username, dev.Email, dev.FullName,
		services.SessionDeveloper,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/dev/login")
		return
	}

	c.SetCookie("dev_session", sessionID, 86400*30, "/", "", false, true)
	c.Redirect(http.StatusFound, "/dev/dashboard")
}

func (dc *DeveloperController) ShowLogin(c *gin.Context) {
	if cookie, err := c.Cookie("dev_session"); err == nil {
		if sess, ok := dc.sessions.Get(cookie); ok && sess.Kind == services.SessionDeveloper {
			c.Redirect(http.StatusFound, "/dev/dashboard")
			return
		}
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_login", gin.H{
		"Error": c.Query("error"),
	})
}

func (dc *DeveloperController) Login(c *gin.Context) {
	login := c.PostForm("login") // username or email
	password := c.PostForm("password")

	var dev models.Developer
	if err := database.DB.
		Where("username = ? OR email = ?", login, login).
		First(&dev).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/login?error=invalid")
		return
	}

	if !dev.CheckPassword(password) {
		c.Redirect(http.StatusFound, "/dev/login?error=invalid")
		return
	}

	sessionID, err := dc.sessions.Create(
		dev.ID, dev.Username, dev.Email, dev.FullName,
		services.SessionDeveloper,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/dev/login?error=server")
		return
	}

	c.SetCookie("dev_session", sessionID, 86400*30, "/", "", false, true)
	c.Redirect(http.StatusFound, "/dev/dashboard")
}

func (dc *DeveloperController) Logout(c *gin.Context) {
	if cookie, err := c.Cookie("dev_session"); err == nil {
		dc.sessions.Destroy(cookie)
	}
	c.SetCookie("dev_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/dev/login")
}

func (dc *DeveloperController) Dashboard(c *gin.Context) {
	devID := dc.getDevID(c)

	var dev models.Developer
	if err := database.DB.First(&dev, "id = ?", devID).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/login")
		return
	}

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
	type VersionRow struct {
		Tag       string
		Signed    bool
		Changelog string
	}
	type PluginRow struct {
		ID             string
		Name           string
		Description    string
		RepoURL        string
		CurrentVersion string
		Status         string
		CreatedAt      string
		UpdatedAt      string
		VersionCount   int
		Versions       []VersionRow
		ScanIssues     int
		HasCritical    bool
	}

	rows := make([]PluginRow, len(plugins))
	for i, p := range plugins {
		vRows := make([]VersionRow, len(p.Versions))
		for j, v := range p.Versions {
			vRows[j] = VersionRow{
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

		rows[i] = PluginRow{
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

	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_dashboard", gin.H{
		"Dev":          dev,
		"Username":     dev.Username,
		"FullName":     dev.FullName,
		"AvatarLetter": dev.AvatarLetter(),
		"Plugins":      rows,
		"TotalPlugins": len(plugins),
		"Pending":      pending,
		"Approved":     approved,
		"Rejected":     rejected,
		"Active":       "dashboard",
	})
}

func (dc *DeveloperController) ShowSubmit(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_submit", gin.H{
		"Username":     dc.getDevUsername(c),
		"FullName":     dc.getDevFullName(c),
		"AvatarLetter": string([]rune(dc.getDevFullName(c))[:1]),
		"Active":       "submit",
		"Error":        c.Query("error"),
		"Success":      c.Query("success"),
	})
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

	type VersionRow struct {
		Tag        string
		Signed     bool
		SHA256Hash string
		Changelog  string
		CreatedAt  string
	}

	vRows := make([]VersionRow, len(plugin.Versions))
	for i, v := range plugin.Versions {
		vRows[i] = VersionRow{
			Tag:        v.Tag,
			Signed:     v.Signature != "",
			SHA256Hash: v.SHA256Hash,
			Changelog:  v.Changelog,
			CreatedAt:  v.CreatedAt.Format("Jan 02, 2006"),
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_plugin_detail", gin.H{
		"Username":     dc.getDevUsername(c),
		"FullName":     dc.getDevFullName(c),
		"AvatarLetter": string([]rune(dc.getDevFullName(c) + "?")[:1]),
		"Plugin":       plugin,
		"PluginStatus": string(plugin.Status),
		"CreatedAt":    plugin.CreatedAt.Format("January 02, 2006 15:04 UTC"),
		"UpdatedAt":    plugin.UpdatedAt.Format("January 02, 2006 15:04 UTC"),
		"ScanReport":   scanReport,
		"Versions":     vRows,
		"Active":       "plugins",
		"DownloadBase": fmt.Sprintf("/v1/plugins/download/%s", plugin.ID),
	})
}

func (dc *DeveloperController) ShowProfile(c *gin.Context) {
	devID := dc.getDevID(c)

	var dev models.Developer
	if err := database.DB.First(&dev, "id = ?", devID).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/login")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	dc.renderer.Render(c.Writer, "dev_profile", gin.H{
		"Username":     dev.Username,
		"FullName":     dev.FullName,
		"AvatarLetter": dev.AvatarLetter(),
		"Dev":          dev,
		"Active":       "profile",
		"Success":      c.Query("success"),
		"Error":        c.Query("error"),
	})
}

func (dc *DeveloperController) UpdateProfile(c *gin.Context) {
	devID := dc.getDevID(c)

	var dev models.Developer
	if err := database.DB.First(&dev, "id = ?", devID).Error; err != nil {
		c.Redirect(http.StatusFound, "/dev/login")
		return
	}

	dev.FullName = c.PostForm("full_name")
	dev.Company = c.PostForm("company")
	dev.Website = c.PostForm("website")
	dev.Bio = c.PostForm("bio")

	// Password change (optional)
	currentPass := c.PostForm("current_password")
	newPass := c.PostForm("new_password")
	confirmPass := c.PostForm("confirm_password")

	if currentPass != "" {
		if !dev.CheckPassword(currentPass) {
			c.Redirect(http.StatusFound, "/dev/profile?error=wrong_password")
			return
		}
		if len(newPass) < 8 {
			c.Redirect(http.StatusFound, "/dev/profile?error=password_short")
			return
		}
		if newPass != confirmPass {
			c.Redirect(http.StatusFound, "/dev/profile?error=password_mismatch")
			return
		}
		dev.SetPassword(newPass)
	}

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
