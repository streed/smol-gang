package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/streed/smol-cluster/gateway/internal/auth"
	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/middleware"
	"github.com/streed/smol-cluster/gateway/internal/models"
)

type AuthHandler struct {
	Queries *db.Queries
	Config  *config.Config
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, h.Config.JWTSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, models.LoginResponse{Token: token, User: user})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "email, password, and name are required"})
		return
	}

	// Allow first user to self-register as admin; otherwise require admin auth
	count, err := h.Queries.CountUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	if count > 0 {
		claims := middleware.GetUserFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			writeJSON(w, http.StatusForbidden, models.ErrorResponse{Error: "only admins can register new users"})
			return
		}
	}

	if req.Role == "" {
		req.Role = "viewer"
	}
	if count == 0 {
		req.Role = "admin"
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	user, err := h.Queries.CreateUser(r.Context(), models.User{
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
		Role:         req.Role,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "user already exists"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, h.Config.JWTSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusCreated, models.LoginResponse{Token: token, User: user})
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
