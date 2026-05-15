package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
	"github.com/mbarek-hani/FluxHUB/services"
	"github.com/mbarek-hani/FluxHUB/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type OAuthController struct {
	oauthConfig *oauth2.Config
	sessions    *services.SessionStore
}

func NewOAuthController(clientID, clientSecret, redirectURL string, sessions *services.SessionStore) *OAuthController {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
	}
	return &OAuthController{
		oauthConfig: conf,
		sessions:    sessions,
	}
}

func (oc *OAuthController) GithubLogin(c *gin.Context) {
	// Generate random state, for simplicity we just use a static string or random uuid
	state := "random-state-string"
	url := oc.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	c.Redirect(http.StatusFound, url)
}

func (oc *OAuthController) GithubCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusFound, "/login?error=github_failed")
		return
	}

	token, err := oc.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=github_failed")
		return
	}

	client := oc.oauthConfig.Client(context.Background(), token)

	// Get user info
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=github_failed")
		return
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Bio   string `json:"bio"`
		Blog  string `json:"blog"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		c.Redirect(http.StatusFound, "/login?error=github_failed")
		return
	}

	// Try to get email if it's null (users can hide email)
	if ghUser.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
			}
			if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary {
						ghUser.Email = e.Email
						break
					}
				}
			}
		}
	}

	githubIDStr := fmt.Sprintf("%d", ghUser.ID)

	var user models.User
	// Find user by GitHub ID, or Email
	if err := database.DB.Where("github_id = ?", githubIDStr).First(&user).Error; err != nil {
		// Not found by GitHub ID. Let's see if there is an account with the same email.
		if ghUser.Email != "" {
			if err := database.DB.Where("email = ?", ghUser.Email).First(&user).Error; err == nil {
				// User exists with this email, let's link the github account
				user.GithubID = githubIDStr
				database.DB.Save(&user)
			}
		}

		if user.ID == "" {
			// Still not found, create new user
			username := ghUser.Login
			// Check if username is taken
			var count int64
			database.DB.Model(&models.User{}).Where("username = ?", username).Count(&count)
			if count > 0 {
				// Append GitHub ID if username is taken
				username = fmt.Sprintf("%s-%s", username, githubIDStr)
			}

			user = models.User{
				Username: username,
				Email:    ghUser.Email,
				GithubID: githubIDStr,
				FullName: ghUser.Name,
				Bio:      ghUser.Bio,
				Website:  ghUser.Blog,
				Role:     models.RoleDeveloper, // Ensure role is developer
				Verified: true,                 // GitHub users can be considered verified
			}
			if err := database.DB.Create(&user).Error; err != nil {
				c.Redirect(http.StatusFound, "/login?error=server")
				return
			}
		}
	}

	// Log the user in
	sessionID, err := oc.sessions.Create(user.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server")
		return
	}

	encryptedSession, err := utils.Encrypt(sessionID)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=server")
		return
	}

	c.SetCookie("flux_session", encryptedSession, 86400, "/", "", false, true)

	if user.Role == models.RoleAdmin {
		c.Redirect(http.StatusFound, "/admin/dashboard")
	} else {
		c.Redirect(http.StatusFound, "/dev/dashboard")
	}
}
