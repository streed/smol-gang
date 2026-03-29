package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/auth"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/middleware"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type AuthHandler struct {
	Queries *db.Queries
	Config  *config.Config
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "not authenticated"})
		return
	}

	user, err := h.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "not authenticated"})
		return
	}

	token, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, h.Config.JWTSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	if h.Config.GitHubClientID == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "GitHub OAuth not configured"})
		return
	}

	// Generate random state
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate state"})
		return
	}
	state := hex.EncodeToString(stateBytes)

	// Store state as a short-lived signed token
	stateToken, err := auth.GenerateToken(uuid.Nil, state, "oauth-state", h.Config.JWTSecret, 10*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate state token"})
		return
	}

	// Callback must hit the gateway directly, not through the frontend proxy
	callbackURL := fmt.Sprintf("http://localhost:%s/api/v1/auth/github/callback", h.Config.Port)

	url := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		h.Config.GitHubClientID,
		callbackURL,
		"read:user,user:email,repo",
		stateToken,
	)

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	// Validate state token
	_, err := auth.ValidateToken(state, h.Config.JWTSecret)
	if err != nil {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for access token
	tokenReqBody := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s",
		h.Config.GitHubClientID, h.Config.GitHubClientSecret, code)

	tokenReq, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(tokenReqBody))
	tokenReq.Header.Set("Accept", "application/json")
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}
	defer tokenResp.Body.Close()

	var tokenResult struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	body, _ := io.ReadAll(tokenResp.Body)
	json.Unmarshal(body, &tokenResult)

	if tokenResult.AccessToken == "" {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=no_access_token", http.StatusTemporaryRedirect)
		return
	}

	// Fetch GitHub user info
	ghUserReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	ghUserReq.Header.Set("Authorization", "Bearer "+tokenResult.AccessToken)
	ghUserReq.Header.Set("Accept", "application/json")

	ghUserResp, err := http.DefaultClient.Do(ghUserReq)
	if err != nil {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=github_api_failed", http.StatusTemporaryRedirect)
		return
	}
	defer ghUserResp.Body.Close()

	var ghUser struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	body, _ = io.ReadAll(ghUserResp.Body)
	json.Unmarshal(body, &ghUser)

	// If email is empty, fetch from emails endpoint
	if ghUser.Email == "" {
		emailReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailReq.Header.Set("Authorization", "Bearer "+tokenResult.AccessToken)
		emailReq.Header.Set("Accept", "application/json")
		emailResp, err := http.DefaultClient.Do(emailReq)
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
			}
			body, _ = io.ReadAll(emailResp.Body)
			json.Unmarshal(body, &emails)
			for _, e := range emails {
				if e.Primary {
					ghUser.Email = e.Email
					break
				}
			}
			if ghUser.Email == "" && len(emails) > 0 {
				ghUser.Email = emails[0].Email
			}
		}
	}

	if ghUser.Email == "" {
		ghUser.Email = fmt.Sprintf("%s@github.com", ghUser.Login)
	}

	if ghUser.Name == "" {
		ghUser.Name = ghUser.Login
	}

	// Look up user by GitHub ID first, then by email, then create
	user, err := h.Queries.GetUserByGitHubID(r.Context(), ghUser.ID)
	if err != nil {
		// Try by email
		user, err = h.Queries.GetUserByEmail(r.Context(), ghUser.Email)
		if err != nil {
			// Create new user — first user is admin
			count, _ := h.Queries.CountUsers(r.Context())
			role := "operator"
			if count == 0 {
				role = "admin"
			}
			user, err = h.Queries.CreateUser(r.Context(), models.User{
				Email:             ghUser.Email,
				PasswordHash:      "",
				Name:              ghUser.Name,
				Role:              role,
				GitHubID:          &ghUser.ID,
				GitHubLogin:       ghUser.Login,
				GitHubAccessToken: tokenResult.AccessToken,
			})
			if err != nil {
				http.Redirect(w, r, h.Config.FrontendURL+"/login?error=create_user_failed", http.StatusTemporaryRedirect)
				return
			}
		}
	}

	// Update GitHub token
	h.Queries.UpdateUserGitHubToken(r.Context(), user.ID, tokenResult.AccessToken, ghUser.Login)

	// Generate JWT
	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, h.Config.JWTSecret, 24*time.Hour)
	if err != nil {
		http.Redirect(w, r, h.Config.FrontendURL+"/login?error=jwt_failed", http.StatusTemporaryRedirect)
		return
	}

	// Redirect to frontend with token
	http.Redirect(w, r, fmt.Sprintf("%s/auth/github/callback?token=%s", h.Config.FrontendURL, token), http.StatusTemporaryRedirect)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
