package controllers

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

type AuthController struct {
	sessions  *services.SessionStore
	templates *template.Template
}

func NewAuthController(sessions *services.SessionStore, tmpl *template.Template) *AuthController {
	return &AuthController{sessions: sessions, templates: tmpl}
}

func (ac *AuthController) ShowLogin(c *gin.Context) {
	// If already logged in, redirect
	if cookie, err := c.Cookie("flux_session"); err == nil {
		if _, ok := ac.sessions.Get(cookie); ok {
			c.Redirect(http.StatusFound, "/admin/dashboard")
			return
		}
	}

	ac.templates.ExecuteTemplate(c.Writer, "login.html", gin.H{
		"Error": c.Query("error"),
	})
}

func (ac *AuthController) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var admin models.Admin
	if err := database.DB.Where("username = ?", username).First(&admin).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/login?error=invalid")
		return
	}

	if !admin.CheckPassword(password) {
		c.Redirect(http.StatusFound, "/admin/login?error=invalid")
		return
	}

	sessionID, err := ac.sessions.Create(admin.ID, admin.Username)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/login?error=server")
		return
	}

	c.SetCookie("flux_session", sessionID, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (ac *AuthController) Logout(c *gin.Context) {
	if cookie, err := c.Cookie("flux_session"); err == nil {
		ac.sessions.Destroy(cookie)
	}
	c.SetCookie("flux_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/login")
}
