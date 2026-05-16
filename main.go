package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mbarek-hani/FluxHUB/controllers"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/routes"
	"github.com/mbarek-hani/FluxHUB/services"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info(fmt.Sprint("No .env file found, relying on system environment variables"))
	}

	cfg := LoadConfig()

	if err := EnsureKeyExists(cfg.PrivateKeyPath); err != nil {
		slog.Error(fmt.Sprintf("Key error: %v", err))
		os.Exit(1)
	}

	gin.SetMode(cfg.GinMode)
	database.Connect()

	gitManager := services.NewGitManager(cfg.StoragePath)
	scanner := services.NewCodeScanner()
	signer, err := services.NewSigner(cfg.PrivateKeyPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Signer error: %v", err))
		os.Exit(1)
	}
	packager := services.NewPackager(cfg.ZipsPath)
	sessionStore := services.NewSessionStore(30 * 24 * time.Hour)

	pluginCtrl := controllers.NewPluginController(gitManager, scanner)
	adminAPICtrl := controllers.NewAdminController(gitManager, signer, packager, scanner)
	downloadCtrl := controllers.NewDownloadController(packager, signer)
	authCtrl := controllers.NewAuthController(sessionStore)
	adminUICtrl := controllers.NewAdminUIController(gitManager, signer, packager, scanner)
	devCtrl := controllers.NewDeveloperController(sessionStore, gitManager, scanner)
	oauthCtrl := controllers.NewOAuthController(cfg.GithubClientID, cfg.GithubClientSecret, cfg.GithubRedirectURL, sessionStore)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routes.SetupRoutes(router, routes.RouterConfig{
		PluginCtrl:   pluginCtrl,
		AdminAPICtrl: adminAPICtrl,
		DownloadCtrl: downloadCtrl,
		AuthCtrl:     authCtrl,
		AdminUICtrl:  adminUICtrl,
		DevCtrl:      devCtrl,
		OAuthCtrl:    oauthCtrl,
		SessionStore: sessionStore,
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info(fmt.Sprintf("FluxHUB on http://localhost%s", addr))
	if err := router.Run(addr); err != nil {
		slog.Error(fmt.Sprintf("FluxHUB failed to start on http://localhost%s", addr))
	}
}
