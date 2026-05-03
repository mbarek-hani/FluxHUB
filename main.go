package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/controllers"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/middleware"
	"github.com/mbarek-hani/FluxHUB/services"
)

// Config centralise la configuration de l'application
type Config struct {
	Port           string
	StoragePath    string
	ZipsPath       string
	PrivateKeyPath string
	AdminToken     string
	GinMode        string
}

// loadConfig charge la configuration depuis les variables d'environnement
func loadConfig() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		StoragePath:    getEnv("STORAGE_PATH", "./storage/clones"),
		ZipsPath:       getEnv("ZIPS_PATH", "./storage/zips"),
		PrivateKeyPath: getEnv("PRIVATE_KEY_PATH", "./keys/nexus_private.pem"),
		AdminToken:     getEnv("ADMIN_API_TOKEN", ""),
		GinMode:        getEnv("GIN_MODE", "debug"),
	}
}

func main() {
	// Charger la configuration
	cfg := loadConfig()

	// Vérifications de sécurité au démarrage
	if cfg.AdminToken == "" {
		log.Fatal("ADMIN_API_TOKEN n'est pas défini. L'API ne peut pas démarrer.")
	}

	// Initialiser la clé privée si elle n'existe pas
	if err := ensureKeyExists(cfg.PrivateKeyPath); err != nil {
		log.Fatalf("Erreur de gestion des clés: %v", err)
	}

	// Configurer le mode Gin
	gin.SetMode(cfg.GinMode)

	// Connexion à la base de données
	database.Connect()

	// Initialisation des Services
	gitManager := services.NewGitManager(cfg.StoragePath)
	scanner := services.NewCodeScanner()

	signer, err := services.NewSigner(cfg.PrivateKeyPath)
	if err != nil {
		log.Fatalf("Impossible d'initialiser le signer RSA: %v", err)
	}

	packager := services.NewPackager(cfg.ZipsPath)

	// Initialisation des Contrôleurs
	pluginCtrl := controllers.NewPluginController(gitManager, scanner)
	adminCtrl := controllers.NewAdminController(gitManager, signer, packager, scanner)
	downloadCtrl := controllers.NewDownloadController(packager, signer)

	//Configuration du Router Gin
	router := gin.New()

	// Middlewares globaux
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.RateLimiter())

	// Headers de sécurité
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	// Groupe de routes v1
	v1 := router.Group("/v1")
	{
		// Routes Publiques
		plugins := v1.Group("/plugins")
		{
			// POST /v1/plugins/submit - Soumettre un nouveau plugin
			plugins.POST("/submit", pluginCtrl.Submit)

			// GET /v1/plugins - Liste des plugins approuvés
			plugins.GET("", pluginCtrl.ListApproved)

			// GET /v1/plugins/download/:id/:version - Télécharger un plugin
			plugins.GET("/download/:id/:version", downloadCtrl.Download)

			// GET /v1/plugins/:id/versions - Versions disponibles
			plugins.GET("/:id/versions", downloadCtrl.GetVersionInfo)

			// GET /v1/plugins/:id/scan - Résultat d'analyse (debug)
			plugins.GET("/:id/scan", pluginCtrl.GetScanResult)
		}

		// Clé publique RSA (pour vérification des signatures)
		v1.GET("/public-key", downloadCtrl.GetPublicKey)

		//Routes Admin (protégées)
		admin := v1.Group("/admin")
		admin.Use(middleware.AdminAuth())
		{
			// GET /v1/admin/review/:id - Revue d'un plugin
			admin.GET("/review/:id", adminCtrl.Review)

			// GET /v1/admin/diff/:id?from=v1.0.0&to=v1.1.0 - Diff entre versions
			admin.GET("/diff/:id", adminCtrl.GetDiff)

			// POST /v1/admin/approve/:id - Approuver un plugin
			admin.POST("/approve/:id", adminCtrl.Approve)

			// POST /v1/admin/reject/:id - Rejeter un plugin
			admin.POST("/reject/:id", adminCtrl.Reject)

			// GET /v1/admin/plugins/pending - Liste des plugins en attente
			admin.GET("/plugins/pending", adminCtrl.ListPending)

			// POST /v1/admin/rescan/:id - Re-scanner un plugin
			admin.POST("/rescan/:id", adminCtrl.RescanPlugin)
		}
	}

	// Route de santé
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "flux-marketplace",
			"version": "1.0.0",
		})
	})

	// Démarrer le serveur
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("FluxHUB démarré sur http://localhost%s", addr)
	log.Printf("Stockage Git: %s", cfg.StoragePath)
	log.Printf("Stockage ZIPs: %s", cfg.ZipsPath)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Impossible de démarrer le serveur: %v", err)
	}
}

// ensureKeyExists génère la paire de clés RSA si elle n'existe pas
func ensureKeyExists(privateKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		log.Printf("Génération d'une nouvelle paire de clés RSA 4096 bits...")

		// Créer le dossier keys/ si nécessaire
		if err := os.MkdirAll("./keys", 0700); err != nil {
			return fmt.Errorf("impossible de créer le dossier keys/: %w", err)
		}

		publicKeyPath := "./keys/nexus_public.pem"
		if err := services.GenerateKeyPair(privateKeyPath, publicKeyPath); err != nil {
			return fmt.Errorf("impossible de générer les clés RSA: %w", err)
		}

		log.Printf("Clés générées:\n  Privée: %s\n  Publique: %s",
			privateKeyPath, publicKeyPath)
		log.Printf("CONSERVEZ LA CLÉ PRIVÉE EN LIEU SÛR ET NE LA COMMITEZ JAMAIS!")
	}
	return nil
}

// getEnv retourne la valeur d'une variable d'env ou une valeur par défaut
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
