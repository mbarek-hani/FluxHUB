package routes

import (
	"github.com/gin-gonic/gin"
)

func setupMarketPlaceRoutes(router *gin.Engine, cfg RouterConfig) {
	v1 := router.Group("/v1")

	//Integrity / crypto
	v1.GET("/public-key", cfg.MarketplaceCtrl.GetPublicKey)

	//Marketplace API (consumed by analytics app clients (Flux))
	mp := v1.Group("/marketplace")
	{
		plugins := mp.Group("/plugins")

		plugins.GET("", cfg.MarketplaceCtrl.ListPlugins)
		plugins.GET("/:id", cfg.MarketplaceCtrl.GetPlugin)
		plugins.GET("/:id/download", cfg.MarketplaceCtrl.DownloadLatest)
		plugins.GET("/:id/updates", cfg.MarketplaceCtrl.CheckUpdate)
	}
}
