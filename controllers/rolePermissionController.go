package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"

	"github.com/gofiber/fiber/v2"
)

// SetRolePermissionsRequest replaces the permissions of one role.
type SetRolePermissionsRequest struct {
	RoleID uint64   `json:"role_id"`
	Domain string   `json:"domain"` // root superadmin only: act inside another domain
	Codes  []string `json:"codes"`  // permission codes; [] removes all permissions from the role
}

// SetRolePermissions replaces a role's permissions with the given codes. A role can only be given
// permissions its domain can see (all of them for shutterx, only the shared ones for a child domain).
func SetRolePermissions(c *fiber.Ctx) error {
	var req SetRolePermissionsRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	var issues []models.PermissionIssue
	if req.RoleID == 0 {
		issues = append(issues, models.PermissionIssue{Path: "role_id", Message: "role_id is required"})
	}
	if req.Codes == nil {
		issues = append(issues, models.PermissionIssue{Path: "codes", Message: "codes is required (use [] to remove all permissions)"})
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", issues)
	}
	for i := range req.Codes {
		req.Codes[i] = strings.TrimSpace(req.Codes[i])
	}

	dom, ok := domainForRequest(c, req.Domain)
	if !ok {
		return nil
	}

	role, err := domain.GetRoleByID(req.RoleID, dom.ID)
	if err != nil || role == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Role not found in this domain",
		})
	}

	issues, err = domain.SetRolePermissions(role, dom, req.Codes)
	if err != nil {
		return permissionServerError(c, "Failed to set role permissions", err, "Failed to update role permissions")
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Some codes are not available, nothing was changed", issues)
	}

	updated, err := domain.GetRoleByID(role.ID, dom.ID)
	if err != nil || updated == nil {
		return permissionServerError(c, "Failed to reload role", err, "Role updated but could not be reloaded")
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Role permissions updated successfully",
		"data":    updated,
	})
}
