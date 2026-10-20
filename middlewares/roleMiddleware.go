package middlewares

import (
	"studio_backend/db"
	"studio_backend/models"

	"github.com/gofiber/fiber/v2"
)

// JWTAuthMiddleware verifies the access token ("Authorization: Bearer <token>")
func JWTAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, failure := claimsFromRequest(c)
		if failure != nil {
			return unauthorized(c, failure.reason, failure.message)
		}

		c.Locals("userID", claims.UserID)
		c.Locals("userEmail", claims.Email)
		c.Locals("roles", claims.Roles)
		c.Locals("domain", claims.Domain)

		return c.Next()
	}
}

// RoleCheckMiddleware verifies if the authenticated user has the required role
func RoleCheckMiddleware(allowedRoleNames ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("userID")

		// If not already authenticated via JWTAuthMiddleware, validate the token here
		if userID == nil {
			claims, failure := claimsFromRequest(c)
			if failure != nil {
				return unauthorized(c, failure.reason, failure.message)
			}
			userID = claims.UserID
			c.Locals("userID", claims.UserID)
			c.Locals("userEmail", claims.Email)
			c.Locals("roles", claims.Roles)
		}

		if db.DB == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusServiceUnavailable,
				"message": "Database service unavailable",
			})
		}

		var user models.User
		err := db.DB.Preload("Domain").Preload("Roles").First(&user, userID).Error
		if err != nil {
			return unauthorized(c, ReasonUserNotFound, "User not found or session invalid")
		}

		if user.Status != "active" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusForbidden,
				"message": "Account is inactive or blocked",
			})
		}

		if user.Domain == nil || user.Domain.Status != models.DomainStatusActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusForbidden,
				"message": "Domain is inactive",
			})
		}

		// The user's own record is the source of truth for the domain, not the token
		c.Locals("domainID", user.DomainID)
		c.Locals("domain", user.Domain.Name)

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
