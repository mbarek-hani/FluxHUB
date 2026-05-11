package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
)

type Renderer interface {
	Render(w interface{ Write([]byte) (int, error) }, name string, data interface{}) error
}

type AuthController struct {
	sessions *services.SessionStore
	renderer Renderer
}

func NewAuthController(sessions *services.SessionStore, renderer Renderer) *AuthController {
	return &AuthController{sessions: sessions, renderer: renderer}
}

func (ac *AuthController) ShowLogin(c *gin.Context) {
	if cookie, err := c.Cookie("flux_session"); err == nil {
		if sess, ok := ac.sessions.Get(cookie); ok && sess.Kind == services.SessionAdmin {
			c.Redirect(http.StatusFound, "/admin/dashboard")
			return
		}
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	ac.renderer.Render(c.Writer, "login", gin.H{
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

	sessionID, err := ac.sessions.Create(admin.ID, admin.Username, "", "", services.SessionAdmin)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/login?error=server")
		return
	}

	c.SetCookie("flux_session", sessionID, 86400*30, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (ac *AuthController) Logout(c *gin.Context) {
	if cookie, err := c.Cookie("flux_session"); err == nil {
		ac.sessions.Destroy(cookie)
	}
	c.SetCookie("flux_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/login")
}
