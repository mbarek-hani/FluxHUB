package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/controllers"
	"github.com/mbarek-hani/FluxHUB/services"
)

type RouterConfig struct {
	AdminAPICtrl    *controllers.AdminController
	AuthCtrl        *controllers.AuthController
	AdminUICtrl     *controllers.AdminUIController
	DevCtrl         *controllers.DeveloperController
	OAuthCtrl       *controllers.OAuthController
	MarketplaceCtrl *controllers.MarketplaceController
	SessionStore    *services.SessionStore
}

func SetupRoutes(router *gin.Engine, cfg RouterConfig) {
	addSecurityHeaders(router)

	router.Static("/static", "./static")

	setupMarketPlaceRoutes(router, cfg)

	setupAuthRoutes(router, cfg)

	setupAdminRoutes(router, cfg)

	setupDeveloperRoutes(router, cfg)

	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "fluxHUB"})
	})
}
