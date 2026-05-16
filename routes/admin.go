package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/middleware"
)

func setupAdminRoutes(router *gin.Engine, cfg RouterConfig) {
	admin := router.Group("/admin")
	admin.Use(middleware.AdminAuth(cfg.SessionStore))
	admin.GET("/dashboard", cfg.AdminUICtrl.Dashboard)
	admin.GET("/plugins", cfg.AdminUICtrl.PluginsList)
	admin.GET("/developers", cfg.AdminUICtrl.DevelopersList)
	admin.GET("/plugins/:id/review", cfg.AdminUICtrl.PluginReview)
	admin.GET("/plugins/:id/browse", cfg.AdminUICtrl.PluginBrowse)
	admin.GET("/plugins/:id/diff", cfg.AdminUICtrl.PluginDiff)

	api := admin.Group("/api")
	api.GET("/plugins/:id/tree", cfg.AdminUICtrl.APIGetFileTree)
	api.GET("/plugins/:id/file", cfg.AdminUICtrl.APIGetFileContent)
	api.GET("/plugins/:id/diff", cfg.AdminUICtrl.APIGetDiff)
	api.POST("/plugins/:id/approve", cfg.AdminUICtrl.APIApprovePlugin)
	api.POST("/plugins/:id/reject", cfg.AdminUICtrl.APIRejectPlugin)
	api.POST("/developers/:id/block", cfg.AdminUICtrl.APIBlockDeveloper)
	api.POST("/developers/:id/unblock", cfg.AdminUICtrl.APIUnblockDeveloper)
}
