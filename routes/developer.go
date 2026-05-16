package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/middleware"
)

func setupDeveloperRoutes(router *gin.Engine, cfg RouterConfig) {
	dev := router.Group("/dev")
	dev.Use(middleware.DeveloperAuth(cfg.SessionStore))
	dev.GET("/dashboard", cfg.DevCtrl.Dashboard)
	dev.GET("/submit", cfg.DevCtrl.ShowSubmit)
	dev.POST("/submit", cfg.DevCtrl.Submit)
	dev.GET("/plugins/:id", cfg.DevCtrl.PluginDetail)
	dev.GET("/profile", cfg.DevCtrl.ShowProfile)
	dev.POST("/profile", cfg.DevCtrl.UpdateProfile)

	dev.GET("/api/plugins/:id/status", cfg.DevCtrl.APIGetPluginStatus)
}
