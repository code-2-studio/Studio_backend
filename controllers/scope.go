package controllers

import (
	"net/http"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"

	"github.com/gofiber/fiber/v2"
)

// currentUser returns the authenticated user loaded by middlewares.RoleCheckMiddleware.
func currentUser(c *fiber.Ctx) (*models.User, bool) {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return nil, false
	}
	return &user, true
}

// domainScope decides which domain a request operates on and returns its id.
//
//   - Everyone works in their own domain by default.
//   - A root (shutterx) superadmin may target a child domain with ?domain=<name>, or, when
//     allowAll is true, every domain with ?domain=all (returned as 0).
//   - Anyone else naming a different domain gets 403.
//
// When ok is false the error response has already been written.
func domainScope(c *fiber.Ctx, allowAll bool) (scope uint64, ok bool) {
	user, authenticated := currentUser(c)
	if !authenticated || user.Domain == nil {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
		return 0, false
	}

	requested := models.NormalizeDomainName(c.Query("domain"))
	if requested == "" || requested == user.Domain.Name {
		return user.DomainID, true
	}

	if !user.IsRootSuperAdmin() {
		_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Forbidden: You cannot access another domain",
		})
		return 0, false
	}

	if requested == "all" {
		if allowAll {
			return 0, true
		}
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Specify one domain by name for this request",
		})
		return 0, false
	}

	target, err := domain.GetDomainByName(strings.TrimSpace(requested))
	if err != nil || target == nil {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Domain not found",
		})
		return 0, false
	}
	return target.ID, true
}
