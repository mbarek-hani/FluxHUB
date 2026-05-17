package routes

import "github.com/gin-gonic/gin"

func setupMarketPlaceRoutes(router *gin.Engine, cfg RouterConfig) {
	v1 := router.Group("/v1")
	v1.GET("/public-key", cfg.DownloadCtrl.GetPublicKey)

	plugins := v1.Group("/plugins")
	plugins.GET("", cfg.PluginCtrl.ListApproved)
	plugins.GET("/download/:id/:version", cfg.DownloadCtrl.Download)
	plugins.GET("/:id/versions", cfg.DownloadCtrl.GetVersionInfo)
	plugins.GET("/:id/scan", cfg.PluginCtrl.GetScanResult)
}
