package controllers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// setupKey is the secret that unlocks first-run setup, set by whoever deploys the server.
// When it is empty the setup call is switched off.
func setupKey() string {
	return strings.TrimSpace(os.Getenv("SETUP_KEY"))
}

// setupKeyMatches compares keys without leaking how much of the key was right.
func setupKeyMatches(provided, expected string) bool {
	if expected == "" {
		return false
	}
	a, b := sha256.Sum256([]byte(provided)), sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// SetupStatus tells the frontend whether to show the first-run setup screen. Public.
func SetupStatus(c *fiber.Ctx) error {
	initialized, err := domain.IsInitialized()
	if err != nil {
		return permissionServerError(c, "Failed to read setup status", err, "Failed to read setup status")
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Setup status",
		"data": fiber.Map{
			"initialized":   initialized,
			"setup_enabled": setupKey() != "",
		},
	})
}

// Setup creates the first superadmin of the shutterx domain, and optionally the first permission
// catalog. Public, but it needs the SETUP_KEY secret and only works until a superadmin exists.
func Setup(c *fiber.Ctx) error {
	expected := setupKey()
	if expected == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Setup is not enabled",
		})
	}

	var req models.SetupRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	// the key is checked before anything else, so a wrong key learns nothing
	if !setupKeyMatches(req.SetupKey, expected) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Invalid setup key",
		})
	}

	if initialized, err := domain.IsInitialized(); err != nil {
		return permissionServerError(c, "Failed to read setup status", err, "Failed to run setup")
	} else if initialized {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "System is already initialized",
		})
	}

	in, issues := models.ParseSetupSuperAdmin(req.SuperAdmin)

	var groups []models.FeatureGroupItem
	isArray := false
	if raw := strings.TrimSpace(string(req.Permissions)); raw != "" && raw != "null" {
		var parseIssues []models.PermissionIssue
		groups, isArray, parseIssues = models.ParsePermissionPayload(req.Permissions)
		for _, is := range parseIssues {
			if is.Path == "" {
				is.Path = "permissions"
			} else {
				is.Path = "permissions" + prefixSetupPath(is.Path)
			}
			issues = append(issues, is)
		}
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was created", issues)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return permissionServerError(c, "Failed to hash superadmin password", err, "Failed to run setup")
	}

	result, conflicts, err := domain.InitializeSystem(in, string(hash), groups, isArray)
	switch {
	case err == domain.ErrAlreadyInitialized:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "System is already initialized",
		})
	case err != nil:
		return permissionServerError(c, "Failed to initialize system", err, "Failed to run setup")
	case len(conflicts) > 0:
		return permissionIssuesResponse(c, permissionIssuesStatus(conflicts), "Setup failed, nothing was created", conflicts)
	}

	login := in.Username + "@" + models.RootDomainName
	utils.Logger.Infow("System initialized", "login", login, "permissions_created", result.PermissionsCreated)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "System initialized",
		"data": fiber.Map{
			"login":               login,
			"user":                result.User,
			"permissions_created": result.PermissionsCreated,
		},
	})
}

// prefixSetupPath makes a path from the permission parser relative to the body's "permissions" field.
func prefixSetupPath(path string) string {
	if strings.HasPrefix(path, "[") {
		return path
	}
	return "[0]." + path
}
