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

type Config struct {
	Port               string
	StoragePath        string
	ZipsPath           string
	PrivateKeyPath     string
	GinMode            string
	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURL  string
}

func loadConfig() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		StoragePath:        getEnv("STORAGE_PATH", "./storage/clones"),
		ZipsPath:           getEnv("ZIPS_PATH", "./storage/zips"),
		PrivateKeyPath:     getEnv("PRIVATE_KEY_PATH", "./keys/flux_hub_private.pem"),
		GinMode:            getEnv("GIN_MODE", "debug"),
		GithubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GithubRedirectURL:  getEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/github/callback"),
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info(fmt.Sprint("No .env file found, relying on system environment variables"))
	}

	cfg := loadConfig()

	if err := ensureKeyExists(cfg.PrivateKeyPath); err != nil {
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
	slog.Info(fmt.Sprintf("Unified Login: http://localhost%s/login", addr))
	router.Run(addr)
}

func ensureKeyExists(privateKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		slog.Info(fmt.Sprint("Generating RSA 4096 key pair..."))
		os.MkdirAll("./keys", 0700)
		publicKeyPath := "./keys/flux_hub_public.pem"
		if err := services.GenerateKeyPair(privateKeyPath, publicKeyPath); err != nil {
			return err
		}
		slog.Info(fmt.Sprintf("Keys generated: %s, %s", privateKeyPath, publicKeyPath))
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
