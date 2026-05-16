package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
	"github.com/mbarek-hani/FluxHUB/views/pages"
)

type AdminUIController struct {
	gitManager *services.GitManager
	signer     *services.Signer
	packager   *services.Packager
	scanner    *services.CodeScanner
}

func NewAdminUIController(
	gm *services.GitManager,
	sg *services.Signer,
	pk *services.Packager,
	sc *services.CodeScanner,
) *AdminUIController {
	return &AdminUIController{
		gitManager: gm,
		signer:     sg,
		packager:   pk,
		scanner:    sc,
	}
}

func (ctrl *AdminUIController) getUsername(c *gin.Context) string {
	if u, ok := c.Get("user_username"); ok {
		return fmt.Sprintf("%v", u)
	}
	return "admin"
}

func (ctrl *AdminUIController) getId(c *gin.Context) string {
	if u, ok := c.Get("user_id"); ok {
		return fmt.Sprintf("%v", u)
	}
	return ""
}

func (ctrl *AdminUIController) getAdmin(c *gin.Context) *models.User {
	var admin models.User
	database.DB.Model(&models.User{}).Where("id = ?", ctrl.getId(c)).First(&admin)
	return &admin
}

func (ctrl *AdminUIController) Dashboard(c *gin.Context) {
	var totalPlugins, pending, approved, rejected int64
	database.DB.Model(&models.Plugin{}).Count(&totalPlugins)
	database.DB.Model(&models.Plugin{}).Where("status = ?", "pending").Count(&pending)
	database.DB.Model(&models.Plugin{}).Where("status = ?", "approved").Count(&approved)
	database.DB.Model(&models.Plugin{}).Where("status = ?", "rejected").Count(&rejected)

	var recentPlugins []models.Plugin
	database.DB.Order("created_at DESC").Limit(10).Find(&recentPlugins)

	// Pre-format dates for each plugin
	rows := make([]pages.AdminPluginRow, len(recentPlugins))
	for i, p := range recentPlugins {
		rows[i] = pages.AdminPluginRow{
			ID:             p.ID,
			Name:           p.Name,
			DeveloperID:    p.DeveloperID,
			CurrentVersion: p.CurrentVersion,
			Status:         string(p.Status),
			CreatedAt:      p.CreatedAt.Format("Jan 02, 15:04"),
		}
	}

	stats := pages.AdminDashboardStats{
		TotalPlugins: int(totalPlugins),
		Pending:      pending,
		Approved:     approved,
		Rejected:     rejected,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminDashboard(ctrl.getUsername(c), ctrl.getAdmin(c).AdminAvatarLetter(), stats, rows).Render(c.Request.Context(), c.Writer)
}

func (ctrl *AdminUIController) PluginsList(c *gin.Context) {
	statusFilter := c.DefaultQuery("status", "all")
	var plugins []models.Plugin
	query := database.DB.Preload("Versions").Order("created_at DESC")
	if statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}
	query.Find(&plugins)

	rows := make([]pages.AdminPluginListRow, len(plugins))
	for i, p := range plugins {
		rows[i] = pages.AdminPluginListRow{
			ID:             p.ID,
			Name:           p.Name,
			Description:    p.Description,
			DeveloperID:    p.DeveloperID,
			CurrentVersion: p.CurrentVersion,
			Status:         string(p.Status),
			VersionCount:   len(p.Versions),
			CreatedAt:      p.CreatedAt.Format("Jan 02, 2006"),
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminPluginsList(ctrl.getUsername(c), ctrl.getAdmin(c).AdminAvatarLetter(), rows, statusFilter).Render(c.Request.Context(), c.Writer)
}

func (ctrl *AdminUIController) DevelopersList(c *gin.Context) {
	searchQuery := c.Query("q")
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	pageSize := 10
	offset := (page - 1) * pageSize

	var developers []models.User
	var total int64

	query := database.DB.Model(&models.User{}).Where("role = ?", models.RoleDeveloper)
	if searchQuery != "" {
		query = query.Where("username LIKE ? OR email LIKE ? OR github_id LIKE ?", "%"+searchQuery+"%", "%"+searchQuery+"%", "%"+searchQuery+"%")
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&developers)

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	rows := make([]pages.AdminDeveloperRow, len(developers))
	for i, d := range developers {
		rows[i] = pages.AdminDeveloperRow{
			ID:        d.ID,
			Username:  d.Username,
			Email:     d.Email,
			GithubID:  d.GithubID,
			AvatarURL: d.AvatarURL,
			IsBlocked: d.IsBlocked,
			JoinedAt:  d.CreatedAt.Format("Jan 02, 2006"),
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminDevelopersList(ctrl.getUsername(c), ctrl.getAdmin(c).AdminAvatarLetter(), rows, searchQuery, page, totalPages).Render(c.Request.Context(), c.Writer)
}

func (ctrl *AdminUIController) PluginReview(c *gin.Context) {
	id := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.Preload("Versions").First(&plugin, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/plugins")
		return
	}

	var scanReport models.ScanReport
	if plugin.ScanResult != "" {
		json.Unmarshal([]byte(plugin.ScanResult), &scanReport)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminPluginReview(
		ctrl.getUsername(c),
		ctrl.getAdmin(c).AdminAvatarLetter(),
		plugin,
		scanReport,
		string(plugin.Status),
		plugin.CreatedAt.Format("January 02, 2006 15:04 UTC"),
	).Render(c.Request.Context(), c.Writer)
}

func (ctrl *AdminUIController) PluginBrowse(c *gin.Context) {
	id := c.Param("id")
	ref := c.DefaultQuery("ref", "")

	var plugin models.Plugin
	if err := database.DB.Preload("Versions").First(&plugin, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/plugins")
		return
	}

	tags, _ := ctrl.gitManager.GetTags(plugin.ID)
	if ref == "" && len(tags) > 0 {
		ref = tags[0]
	} else if ref == "" {
		ref = "HEAD"
	}

	tagsJSON, _ := json.Marshal(tags)

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminPluginBrowse(
		ctrl.getUsername(c),
		ctrl.getAdmin(c).AdminAvatarLetter(),
		plugin,
		ref,
		string(tagsJSON),
	).Render(c.Request.Context(), c.Writer)
}

func (ctrl *AdminUIController) PluginDiff(c *gin.Context) {
	id := c.Param("id")

	var plugin models.Plugin
	if err := database.DB.Preload("Versions").First(&plugin, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/plugins")
		return
	}

	tags, _ := ctrl.gitManager.GetTags(plugin.ID)
	tagsJSON, _ := json.Marshal(tags)

	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.AdminPluginDiff(
		ctrl.getUsername(c),
		ctrl.getAdmin(c).AdminAvatarLetter(),
		plugin,
		c.Query("from"),
		c.Query("to"),
		string(tagsJSON),
	).Render(c.Request.Context(), c.Writer)
}

// ---- AJAX API ----

func (ctrl *AdminUIController) APIGetFileTree(c *gin.Context) {
	id := c.Param("id")
	ref := c.DefaultQuery("ref", "HEAD")
	tree, err := ctrl.gitManager.GetFileTree(id, ref)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tree)
}

func (ctrl *AdminUIController) APIGetFileContent(c *gin.Context) {
	id := c.Param("id")
	ref := c.DefaultQuery("ref", "HEAD")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	content, err := ctrl.gitManager.GetFileContent(id, ref, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": filePath, "content": content})
}

func (ctrl *AdminUIController) APIGetDiff(c *gin.Context) {
	id := c.Param("id")
	fromRef := c.Query("from")
	toRef := c.Query("to")
	if fromRef == "" || toRef == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to required"})
		return
	}
	diff, err := ctrl.gitManager.GenerateDiff(id, fromRef, toRef)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"diff": diff})
}

func (ctrl *AdminUIController) APIApprovePlugin(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Version string `json:"version" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var plugin models.Plugin
	if err := database.DB.First(&plugin, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	var version models.Version
	if err := database.DB.Where("plugin_id = ? AND tag = ?", id, req.Version).First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}

	if err := ctrl.gitManager.CheckoutTag(id, req.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "checkout failed: " + err.Error()})
		return
	}

	repoPath := ctrl.gitManager.GetRepoPathForPlugin(id)
	zipPath, err := ctrl.packager.PackagePlugin(repoPath, id, req.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "packaging failed: " + err.Error()})
		return
	}

	signature, sha256Hash, err := ctrl.signer.SignFile(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "signing failed: " + err.Error()})
		return
	}

	database.DB.Model(&version).Updates(map[string]interface{}{
		"signature":   signature,
		"sha256_hash": sha256Hash,
		"zip_path":    zipPath,
		"changelog":   req.Comment,
	})

	database.DB.Model(&plugin).Updates(map[string]interface{}{
		"status":          models.StatusApproved,
		"current_version": req.Version,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Plugin approved", "sha256": sha256Hash})
}

func (ctrl *AdminUIController) APIRejectPlugin(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Model(&models.Plugin{}).Where("id = ?", id).Update("status", models.StatusRejected)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin rejected"})
}

func (ctrl *AdminUIController) APIBlockDeveloper(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Model(&models.User{}).Where("id = ? AND role = ?", id, models.RoleDeveloper).Update("is_blocked", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block developer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Developer blocked"})
}

func (ctrl *AdminUIController) APIUnblockDeveloper(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Model(&models.User{}).Where("id = ? AND role = ?", id, models.RoleDeveloper).Update("is_blocked", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock developer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Developer unblocked"})
}
