package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
	"github.com/mbarek-hani/FluxHUB/utils"
	"github.com/mbarek-hani/FluxHUB/views/pages"
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
		if decryptedCookie, err := utils.Decrypt(cookie); err == nil {
			if user, ok := ac.sessions.Get(decryptedCookie); ok {
				if user.Role == models.RoleAdmin {
					c.Redirect(http.StatusFound, "/admin/dashboard")
				} else {
					c.Redirect(http.StatusFound, "/dev/dashboard")
				}
				return
			}
		}
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	pages.Login(c.Query("error")).Render(c.Request.Context(), c.Writer)
}

func (ac *AuthController) Login(c *gin.Context) {
	login := c.PostForm("login") // username or email
	if login == "" {
		login = c.PostForm("username") // fallback for old form
	}
	password := c.PostForm("password")

	var user models.User
	if err := database.DB.Where("username = ? OR email = ?", login, login).First(&user).Error; err != nil {
		c.Redirect(http.StatusFound, "/login?error=invalid")
		return
	}

	if !user.CheckPassword(password) {
		c.Redirect(http.StatusFound, "/login?error=invalid")
		return
	}

	sessionID, err := ac.sessions.Create(user.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server")
		return
	}

	encryptedSession, err := utils.Encrypt(sessionID)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server")
		return
	}

	// 1-day expiration (86400 seconds)
	c.SetCookie("flux_session", encryptedSession, 86400, "/", "", false, true)

	if user.Role == models.RoleAdmin {
		c.Redirect(http.StatusFound, "/admin/dashboard")
	} else {
		c.Redirect(http.StatusFound, "/dev/dashboard")
	}
}

func (ac *AuthController) Logout(c *gin.Context) {
	if cookie, err := c.Cookie("flux_session"); err == nil {
		if decryptedCookie, err := utils.Decrypt(cookie); err == nil {
			ac.sessions.Destroy(decryptedCookie)
		}
	}
	c.SetCookie("flux_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
