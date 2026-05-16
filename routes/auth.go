package routes

import "github.com/gin-gonic/gin"

func setupAuthRoutes(router *gin.Engine, cfg RouterConfig) {
	router.GET("/login", cfg.AuthCtrl.ShowDevLogin)
	router.GET("/admin/login", cfg.AuthCtrl.ShowAdminLogin)
	router.POST("/admin/login", cfg.AuthCtrl.Login)
	router.POST("/logout", cfg.AuthCtrl.Logout)

	router.GET("/auth/github", cfg.OAuthCtrl.GithubLogin)
	router.GET("/auth/github/callback", cfg.OAuthCtrl.GithubCallback)
}
