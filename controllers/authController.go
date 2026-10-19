package controllers

import (
	"net/http"
	"strings"
	"time"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"`
}

// DeviceInfo describes the client device used to log in.
// It is optional; today it is only recorded, later it can be used to limit logins per device.
type DeviceInfo struct {
	DeviceID       string `json:"device_id"`
	DeviceType     string `json:"device_type"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	UserAgent      string `json:"user_agent"`
}

type LoginRequest struct {
	Email      string      `json:"email"`
	Password   string      `json:"password"`
	DeviceInfo *DeviceInfo `json:"device_info,omitempty"`
}

// truncate caps client-supplied strings so they fit the column sizes.
func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}

// recordLoginDevice stores the device used for this login and returns the response
// block describing it (nil if no device was sent). Failures are logged, never block login.
func recordLoginDevice(c *fiber.Ctx, userID uint64, info *DeviceInfo) fiber.Map {
	if info == nil || strings.TrimSpace(info.DeviceID) == "" {
		return nil
	}

	userAgent := info.UserAgent
	if userAgent == "" {
		userAgent = c.Get(fiber.HeaderUserAgent)
	}

	device := models.UserDevice{
		UserID:         userID,
		DeviceID:       truncate(info.DeviceID, 100),
		DeviceType:     truncate(info.DeviceType, 30),
		OS:             truncate(info.OS, 50),
		OSVersion:      truncate(info.OSVersion, 50),
		Browser:        truncate(info.Browser, 50),
		BrowserVersion: truncate(info.BrowserVersion, 50),
		UserAgent:      truncate(userAgent, 512),
		IPAddress:      truncate(c.IP(), 45),
	}

	isNew, err := domain.RecordUserDevice(&device)
	if err != nil {
		utils.Logger.Errorw("Failed to record login device", "user_id", userID, "error", err)
		return nil
	}

	return fiber.Map{
		"device_id":     device.DeviceID,
		"is_new_device": isNew,
	}
}

// Register creates a new user account
func Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Name and email are required",
		})
	}

	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Password must be at least 8 characters long",
		})
	}

	// Check if user already exists
	existing, _ := domain.GetUserByEmail(req.Email)
	if existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Email is already registered",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Logger.Errorw("Failed to hash password", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to process user registration",
		})
	}

	newUser := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: string(hashedPassword),
		Status:   "active",
	}

	if err := domain.CreateUser(&newUser); err != nil {
		utils.Logger.Errorw("Failed to create user", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create user account",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "User registered successfully",
		"data":    newUser.ToResponse(),
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Login authenticates a user and returns an access token and refresh token
func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Email and password are required",
		})
	}

	user, err := domain.GetUserByEmail(req.Email)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Invalid email or password",
		})
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Account is inactive or blocked",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Invalid email or password",
		})
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	_ = domain.UpdateUser(user)

	// Record the device used for this login (no device limit enforced yet)
	deviceResult := recordLoginDevice(c, user.ID, req.DeviceInfo)

	// Collect user role names
	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}

	// Generate access token & refresh token
	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, roleNames)
	if err != nil {
		utils.Logger.Errorw("Failed to generate token pair", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to generate authentication tokens",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Login successful",
		"data": fiber.Map{
			"user":   user.ToResponse(),
			"tokens": tokens,
			"device": deviceResult,
		},
	})
}

// RefreshToken validates a refresh token and issues a new access & refresh token pair
func RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "refresh_token is required",
		})
	}

	claims, err := utils.ValidateToken(req.RefreshToken, utils.TokenTypeRefresh)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Invalid or expired refresh token",
		})
	}

	user, err := domain.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "User not found or session terminated",
		})
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Account is inactive or blocked",
		})
	}

	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}

	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, roleNames)
	if err != nil {
		utils.Logger.Errorw("Failed to generate token pair on refresh", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to refresh authentication tokens",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Token refreshed successfully",
		"data": fiber.Map{
			"tokens": tokens,
		},
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Logout invalidates the active access token and/or refresh token
func Logout(c *fiber.Ctx) error {
	// 1. Invalidate Access Token from Authorization Header if present
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			accessToken := strings.TrimSpace(parts[1])
			utils.RevokeToken(accessToken)
		}
	}

	// 2. Invalidate Refresh Token if provided in body
	var req LogoutRequest
	if err := c.BodyParser(&req); err == nil && req.RefreshToken != "" {
		utils.RevokeToken(strings.TrimSpace(req.RefreshToken))
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Logged out successfully",
	})
}
