package middlewares

import (
	"studio_backend/models"

	"github.com/gofiber/fiber/v2"
)

// RootSuperAdminMiddleware allows only a superadmin of the root (shutterx) domain.
// It must run after RoleCheckMiddleware, which loads the user, their domain and roles.
func RootSuperAdminMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(models.User)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusUnauthorized,
				"message": "Unauthorized: Authentication required",
			})
		}

		if !user.IsRootSuperAdmin() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusForbidden,
				"message": "Forbidden: Only the root domain superadmin can do this",
			})
		}

		return c.Next()
	}
}
