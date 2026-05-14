package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/controllers"
	"github.com/mbarek-hani/FluxHUB/middleware"
	"github.com/mbarek-hani/FluxHUB/services"
)

type RouterConfig struct {
	AdminToken   string
	PluginCtrl   *controllers.PluginController
	AdminAPICtrl *controllers.AdminController
	DownloadCtrl *controllers.DownloadController
	AuthCtrl     *controllers.AuthController
	AdminUICtrl  *controllers.AdminUIController
	DevCtrl      *controllers.DeveloperController
	SessionStore *services.SessionStore
}

func SetupRoutes(router *gin.Engine, cfg RouterConfig) {
	// Security headers
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	router.Static("/static", "./static")

	// ---- Public API v1 ----
	v1 := router.Group("/v1")
	{
		plugins := v1.Group("/plugins")
		{
			plugins.POST("/submit", cfg.PluginCtrl.Submit)
			plugins.GET("", cfg.PluginCtrl.ListApproved)
			plugins.GET("/download/:id/:version", cfg.DownloadCtrl.Download)
			plugins.GET("/:id/versions", cfg.DownloadCtrl.GetVersionInfo)
			plugins.GET("/:id/scan", cfg.PluginCtrl.GetScanResult)
		}
		v1.GET("/public-key", cfg.DownloadCtrl.GetPublicKey)

		// Token-protected API
		if cfg.AdminToken != "" {
			adminAPI := v1.Group("/admin")
			adminAPI.Use(middleware.AdminAuth())
			{
				adminAPI.GET("/review/:id", cfg.AdminAPICtrl.Review)
				adminAPI.GET("/diff/:id", cfg.AdminAPICtrl.GetDiff)
				adminAPI.POST("/approve/:id", cfg.AdminAPICtrl.Approve)
				adminAPI.POST("/reject/:id", cfg.AdminAPICtrl.Reject)
				adminAPI.GET("/plugins/pending", cfg.AdminAPICtrl.ListPending)
				adminAPI.POST("/rescan/:id", cfg.AdminAPICtrl.RescanPlugin)
			}
		}
	}

	// ---- Unified Auth ----
	router.GET("/login", cfg.AuthCtrl.ShowLogin)
	router.POST("/login", cfg.AuthCtrl.Login)
	router.POST("/logout", cfg.AuthCtrl.Logout)

	// ---- Admin UI (session-based) ----
	admin := router.Group("/admin")
	{

		// Protected UI routes
		protected := admin.Group("")
		protected.Use(middleware.SessionAuth(cfg.SessionStore))
		{
			protected.GET("/dashboard", cfg.AdminUICtrl.Dashboard)
			protected.GET("/plugins", cfg.AdminUICtrl.PluginsList)
			protected.GET("/plugins/:id/review", cfg.AdminUICtrl.PluginReview)
			protected.GET("/plugins/:id/browse", cfg.AdminUICtrl.PluginBrowse)
			protected.GET("/plugins/:id/diff", cfg.AdminUICtrl.PluginDiff)

			// AJAX API for UI
			api := protected.Group("/api")
			{
				api.GET("/plugins/:id/tree", cfg.AdminUICtrl.APIGetFileTree)
				api.GET("/plugins/:id/file", cfg.AdminUICtrl.APIGetFileContent)
				api.GET("/plugins/:id/diff", cfg.AdminUICtrl.APIGetDiff)
				api.POST("/plugins/:id/approve", cfg.AdminUICtrl.APIApprovePlugin)
				api.POST("/plugins/:id/reject", cfg.AdminUICtrl.APIRejectPlugin)
			}
		}
	}

	// Developer Portal
	dev := router.Group("/dev")
	{
		dev.GET("/register", cfg.DevCtrl.ShowRegister)
		dev.POST("/register", cfg.DevCtrl.Register)

		protected := dev.Group("")
		protected.Use(middleware.DeveloperAuth(cfg.SessionStore))
		{
			protected.GET("/dashboard", cfg.DevCtrl.Dashboard)
			protected.GET("/submit", cfg.DevCtrl.ShowSubmit)
			protected.POST("/submit", cfg.DevCtrl.Submit)
			protected.GET("/plugins/:id", cfg.DevCtrl.PluginDetail)
			protected.GET("/profile", cfg.DevCtrl.ShowProfile)
			protected.POST("/profile", cfg.DevCtrl.UpdateProfile)

			// AJAX
			protected.GET("/api/plugins/:id/status", cfg.DevCtrl.APIGetPluginStatus)
		}
	}

	// Root redirect
	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})

	// Health
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "flux-marketplace"})
	})
}
