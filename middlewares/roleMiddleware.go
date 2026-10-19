package middlewares

import (
	"strings"

	"studio_backend/db"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

// JWTAuthMiddleware verifies the Bearer access token
func JWTAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": "Missing Authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": "Invalid Authorization header format. Expected 'Bearer <token>'",
			})
		}

		tokenString := strings.TrimSpace(parts[1])
		claims, err := utils.ValidateToken(tokenString, utils.TokenTypeAccess)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": err.Error(),
			})
		}

		c.Locals("userID", claims.UserID)
		c.Locals("userEmail", claims.Email)
		c.Locals("roles", claims.Roles)

		return c.Next()
	}
}

// RoleCheckMiddleware verifies if the authenticated user has the required role
func RoleCheckMiddleware(allowedRoleNames ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID")

		// If not already authenticated via JWTAuthMiddleware, attempt to extract token
		if userID == nil {
			authHeader := c.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenString := strings.TrimSpace(parts[1])
					if claims, err := utils.ValidateToken(tokenString, utils.TokenTypeAccess); err == nil {
						userID = claims.UserID
						c.Locals("userID", claims.UserID)
						c.Locals("userEmail", claims.Email)
						c.Locals("roles", claims.Roles)
					}
				}
			}
		}

		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": "Unauthorized: Authentication required",
			})
		}

		if db.DB == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusServiceUnavailable,
				"message": "Database service unavailable",
			})
		}

		var user models.User
		err := db.DB.Preload("Roles").First(&user, userID).Error
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": "User not found or session invalid",
			})
		}

		if user.Status != "active" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusForbidden,
				"message": "Account is inactive or blocked",
			})
		}

		// If no specific roles required, allow active user
		if len(allowedRoleNames) == 0 {
			c.Locals("user", user)
			return c.Next()
		}

		// Check role match
		hasRole := false
		for _, role := range user.Roles {
			for _, allowed := range allowedRoleNames {
				if role.Name == allowed {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusForbidden,
				"message": "Forbidden: Insufficient role permissions",
			})
		}

		c.Locals("user", user)
		return c.Next()
	}
}
