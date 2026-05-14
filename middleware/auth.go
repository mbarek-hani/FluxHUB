package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
	"github.com/mbarek-hani/FluxHUB/utils"
)

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth format"})
			return
		}
		if parts[1] != os.Getenv("ADMIN_API_TOKEN") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid token"})
			return
		}
		c.Next()
	}
}

// SessionAuth protects admin UI routes
func SessionAuth(sessions *services.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("flux_session")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		decryptedCookie, err := utils.Decrypt(cookie)
		if err != nil {
			c.SetCookie("flux_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user, ok := sessions.Get(decryptedCookie)
		if !ok || user.Role != models.RoleAdmin {
			c.SetCookie("flux_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("user_id", user.ID)
		c.Set("user_username", user.Username)
		c.Set("user_role", string(user.Role))
		c.Next()
	}
}

// DeveloperAuth protects developer portal routes
func DeveloperAuth(sessions *services.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("flux_session")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		decryptedCookie, err := utils.Decrypt(cookie)
		if err != nil {
			c.SetCookie("flux_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user, ok := sessions.Get(decryptedCookie)
		if !ok || user.Role != models.RoleDeveloper {
			c.SetCookie("flux_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("user_id", user.ID)
		c.Set("user_username", user.Username)
		c.Set("user_role", string(user.Role))
		c.Set("dev_email", user.Email)
		c.Set("dev_fullname", user.FullName)
		c.Next()
	}
}
