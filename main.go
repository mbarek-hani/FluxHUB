package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mbarek-hani/FluxHUB/controllers"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/routes"
	"github.com/mbarek-hani/FluxHUB/services"
)

type Config struct {
	Port           string
	StoragePath    string
	ZipsPath       string
	PrivateKeyPath string
	AdminToken     string
	GinMode        string
}

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

// TemplateRegistry holds per-page compiled templates
type TemplateRegistry struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateRegistry() *TemplateRegistry {
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"avatarLetter": func(s string) string {
			r := []rune(s)
			if len(r) > 0 {
				return strings.ToUpper(string(r[0]))
			}
			return "?"
		},
		"jsonTags": func(tags []string) template.JS {
			b, _ := json.Marshal(tags)
			return template.JS(b)
		},
		"fmtDate": func(t time.Time) string {
			return t.Format("Jan 02, 15:04")
		},
		"fmtDateLong": func(t time.Time) string {
			return t.Format("January 02, 2006 15:04 UTC")
		},
		"fmtDateShort": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
	}

	reg := &TemplateRegistry{
		templates: make(map[string]*template.Template),
		funcMap:   funcMap,
	}

	// ---- Standalone pages (no layout) ----
	standalones := map[string]string{
		"login":        "templates/login.html",
		"dev_login":    "templates/dev_login.html",
		"dev_register": "templates/dev_register.html",
	}
	for name, file := range standalones {
		reg.templates[name] = template.Must(
			template.New(name).Funcs(funcMap).ParseFiles(file),
		)
	}

	// ---- Admin layout pages ----
	adminPages := []string{
		"dashboard",
		"plugins_list",
		"plugin_review",
		"plugin_browse",
		"plugin_diff",
	}
	for _, page := range adminPages {
		reg.templates[page] = template.Must(
			template.New(page).Funcs(funcMap).ParseFiles(
				"templates/layout.html",
				fmt.Sprintf("templates/%s.html", page),
			),
		)
	}

	// ---- Developer layout pages ----
	devPages := []string{
		"dev_dashboard",
		"dev_submit",
		"dev_plugin_detail",
		"dev_profile",
	}
	for _, page := range devPages {
		reg.templates[page] = template.Must(
			template.New(page).Funcs(funcMap).ParseFiles(
				"templates/dev_layout.html",
				fmt.Sprintf("templates/%s.html", page),
			),
		)
	}

	return reg
}

func (tr *TemplateRegistry) Render(w interface{ Write([]byte) (int, error) }, name string, data interface{}) error {
	tmpl, ok := tr.templates[name]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}

	// Standalone pages
	standalones := map[string]string{
		"login":        "login",
		"dev_login":    "dev_login",
		"dev_register": "dev_register",
	}
	if block, ok := standalones[name]; ok {
		return tmpl.ExecuteTemplate(w, block, data)
	}

	// Developer portal pages use dev_layout
	devPages := map[string]bool{
		"dev_dashboard": true, "dev_submit": true,
		"dev_plugin_detail": true, "dev_profile": true,
	}
	if devPages[name] {
		return tmpl.ExecuteTemplate(w, "dev_layout", data)
	}

	// Admin pages use layout
	return tmpl.ExecuteTemplate(w, "layout", data)
}

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info(fmt.Sprint("No .env file found, relying on system environment variables"))
	}

	cfg := loadConfig()

	if cfg.AdminToken == "" {
		slog.Info(fmt.Sprint("ADMIN_API_TOKEN not set. API token auth disabled."))
	}

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

	renderer := NewTemplateRegistry()

	pluginCtrl := controllers.NewPluginController(gitManager, scanner)
	adminAPICtrl := controllers.NewAdminController(gitManager, signer, packager, scanner)
	downloadCtrl := controllers.NewDownloadController(packager, signer)
	authCtrl := controllers.NewAuthController(sessionStore, renderer)
	adminUICtrl := controllers.NewAdminUIController(gitManager, signer, packager, scanner, renderer)
	devCtrl := controllers.NewDeveloperController(sessionStore, renderer, gitManager, scanner)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routes.SetupRoutes(router, routes.RouterConfig{
		AdminToken:   cfg.AdminToken,
		PluginCtrl:   pluginCtrl,
		AdminAPICtrl: adminAPICtrl,
		DownloadCtrl: downloadCtrl,
		AuthCtrl:     authCtrl,
		AdminUICtrl:  adminUICtrl,
		DevCtrl:      devCtrl,
		SessionStore: sessionStore,
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info(fmt.Sprintf("FluxHUB on http://localhost%s", addr))
	slog.Info(fmt.Sprintf("Admin UI: http://localhost%s/admin/login", addr))
	router.Run(addr)
}

func ensureKeyExists(privateKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		slog.Info(fmt.Sprint("Generating RSA 4096 key pair..."))
		os.MkdirAll("./keys", 0700)
		publicKeyPath := "./keys/nexus_public.pem"
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
