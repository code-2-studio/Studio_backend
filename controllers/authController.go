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
	Domain   string  `json:"domain"`
	Username string  `json:"username"`
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

// LoginRequest identifies the user as "username@domain" (e.g. superadmin@shutterx) in Login,
// or as separate Username + Domain fields. Email is a legacy alias for Login; a real email
// address (its host contains a dot) is looked up inside the root domain.
type LoginRequest struct {
	Login      string      `json:"login"`
	Username   string      `json:"username"`
	Domain     string      `json:"domain"`
	Email      string      `json:"email"`
	Password   string      `json:"password"`
	DeviceInfo *DeviceInfo `json:"device_info,omitempty"`
}

type loginIdentity struct {
	Domain   string
	Username string // set when logging in by username
	Email    string // set when logging in by real email address
}

func parseLoginIdentity(req LoginRequest) (loginIdentity, bool) {
	if req.Username != "" || req.Domain != "" {
		id := loginIdentity{
			Domain:   models.NormalizeDomainName(req.Domain),
			Username: models.NormalizeUsername(req.Username),
		}
		return id, id.Domain != "" && id.Username != ""
	}

	raw := strings.ToLower(strings.TrimSpace(req.Login))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(req.Email))
	}

	at := strings.LastIndex(raw, "@")
	if at <= 0 || at == len(raw)-1 {
		return loginIdentity{}, false
	}

	local, host := raw[:at], raw[at+1:]
	if strings.Contains(host, ".") {
		// domain names never contain dots, so this is a real email address (existing users)
		return loginIdentity{Domain: models.RootDomainName, Email: raw}, true
	}
	return loginIdentity{Domain: host, Username: local}, true
}

// truncate caps client-supplied strings so they fit the column sizes.
func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}

// LoginDevice is the device block returned in the login response.
type LoginDevice struct {
	DeviceID    string `json:"device_id"`
	IsNewDevice bool   `json:"is_new_device"`
}

// recordLoginDevice stores the device used for this login and returns the response
// block describing it (nil if no device was sent). Failures are logged, never block login.
func recordLoginDevice(c *fiber.Ctx, domainID, userID uint64, info *DeviceInfo) *LoginDevice {
	if info == nil || strings.TrimSpace(info.DeviceID) == "" {
		return nil
	}

	userAgent := info.UserAgent
	if userAgent == "" {
		userAgent = c.Get(fiber.HeaderUserAgent)
	}

	device := models.UserDevice{
		DomainID:       domainID,
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

	return &LoginDevice{DeviceID: device.DeviceID, IsNewDevice: isNew}
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

	domainName := models.NormalizeDomainName(req.Domain)
	if domainName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Domain is required",
		})
	}

	dom, err := domain.GetDomainByName(domainName)
	if err != nil || dom == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Domain not found",
		})
	}

	// Root domain accounts are created by the superadmin, never through public sign-up
	if dom.IsRoot() || dom.Status != models.DomainStatusActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Registration is not open for this domain",
		})
	}

	username := models.NormalizeUsername(req.Username)
	if username == "" {
		username = models.DeriveUsername(req.Email)
	}
	if !models.ValidUsername(username) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Username must be 2-50 characters: lowercase letters, digits, '.', '_' or '-'",
		})
	}

	// Check if user already exists in this domain
	if existing, _ := domain.GetUserByEmail(dom.ID, req.Email); existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Email is already registered",
		})
	}
	if existing, _ := domain.GetUserByUsername(dom.ID, username); existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Username is already taken",
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
		DomainID: dom.ID,
		Username: username,
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
	newUser.Domain = dom

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "User registered successfully",
		"data":    newUser.ToResponse(),
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id,omitempty"`
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

	identity, valid := parseLoginIdentity(req)
	if !valid || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Login (username@domain) and password are required",
		})
	}

	// Unknown domain, unknown user and wrong password all give the same answer
	invalidLogin := func() error {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Invalid login or password",
		})
	}

	dom, err := domain.GetDomainByName(identity.Domain)
	if err != nil || dom == nil {
		return invalidLogin()
	}

	var user *models.User
	if identity.Email != "" {
		user, err = domain.GetUserByEmail(dom.ID, identity.Email)
	} else {
		user, err = domain.GetUserByUsername(dom.ID, identity.Username)
	}
	if err != nil || user == nil {
		return invalidLogin()
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Account is inactive or blocked",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return invalidLogin()
	}

	// Checked after the password so an inactive domain is not revealed to someone who cannot log in
	if dom.Status != models.DomainStatusActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Domain is inactive",
		})
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	_ = domain.UpdateUser(user)

	// Record the device used for this login (no device limit enforced yet)
	deviceResult := recordLoginDevice(c, user.DomainID, user.ID, req.DeviceInfo)
	deviceID := ""
	if deviceResult != nil {
		deviceID = deviceResult.DeviceID
	}

	// Collect user role names
	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}

	// Generate access token & refresh token (bound to the device)
	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, roleNames, deviceID, dom.Name)
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

	// The refresh token is bound to the device it was issued for
	reqDeviceID := truncate(req.DeviceID, 100)
	if claims.DeviceID != "" {
		if reqDeviceID != "" && reqDeviceID != claims.DeviceID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusForbidden,
				"message": "Refresh token does not belong to this device",
			})
		}

		if loggedOut, _ := domain.IsDeviceLoggedOut(claims.UserID, claims.DeviceID); loggedOut {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusUnauthorized,
				"message": "Device session terminated, please log in again",
			})
		}
	}

	user, err := domain.GetUserByID(claims.UserID, 0)
	if err != nil || user == nil || user.Domain == nil {
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

	if user.Domain.Status != models.DomainStatusActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Domain is inactive",
		})
	}

	roleNames := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}

	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, roleNames, claims.DeviceID, user.Domain.Name)
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
			"device": fiber.Map{"device_id": claims.DeviceID},
		},
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Logout invalidates the active access token and/or refresh token and marks the device logged out
func Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	hasBody := c.BodyParser(&req) == nil
	refreshToken := ""
	if hasBody {
		refreshToken = strings.TrimSpace(req.RefreshToken)
	}

	accessToken := ""
	if authHeader := c.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			accessToken = strings.TrimSpace(parts[1])
		}
	}

	// Work out which user/device is logging out before the tokens are revoked
	var userID uint64
	deviceID := ""
	if accessToken != "" {
		if claims, err := utils.ValidateToken(accessToken, utils.TokenTypeAccess); err == nil {
			userID, deviceID = claims.UserID, claims.DeviceID
		}
	}
	if deviceID == "" && refreshToken != "" {
		if claims, err := utils.ValidateToken(refreshToken, utils.TokenTypeRefresh); err == nil {
			userID, deviceID = claims.UserID, claims.DeviceID
		}
	}

	utils.RevokeToken(accessToken)
	utils.RevokeToken(refreshToken)

	deviceLoggedOut := false
	if deviceID != "" {
		var err error
		deviceLoggedOut, err = domain.MarkDeviceLoggedOut(userID, deviceID)
		if err != nil {
			utils.Logger.Errorw("Failed to mark device logged out", "user_id", userID, "error", err)
		}
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Logged out successfully",
		"data": fiber.Map{
			"device_id":         deviceID,
			"device_logged_out": deviceLoggedOut,
		},
	})
}
