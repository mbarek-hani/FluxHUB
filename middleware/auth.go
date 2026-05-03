package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminAuth vérifie que la requête provient d'un administrateur
// Implémentation simple par token Bearer - TODO: à remplacer par JWT
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Header Authorization manquant",
			})
			return
		}

		// Vérifier le format "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Format d'autorisation invalide. Utilisez: Bearer <token>",
			})
			return
		}

		token := parts[1]
		adminToken := os.Getenv("ADMIN_API_TOKEN")

		if adminToken == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Token administrateur non configuré",
			})
			return
		}

		if token != adminToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Token administrateur invalide",
			})
			return
		}

		c.Next()
	}
}

// TODO: RateLimiter - Middleware simple de rate limiting
func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implémentation basique - utiliser golang.org/x/time/rate
		c.Next()
	}
}

// TODO: RequestLogger logge les requêtes entrantes
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
