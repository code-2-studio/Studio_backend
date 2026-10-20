package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

// CreateRoleRequest creates a role, optionally with its permissions. Domain (shutterx superadmin
// only) creates it inside another domain; leave it out to create in your own.
type CreateRoleRequest struct {
	Domain      string   `json:"domain"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Codes       []string `json:"codes"` // optional permission codes to give the role
}

// UpdateRoleRequest changes a role. Every field except role_id is optional; send what should change.
type UpdateRoleRequest struct {
	RoleID      uint64    `json:"role_id"`
	Domain      string    `json:"domain"` // shutterx superadmin only
	Name        *string   `json:"name"`
	Description *string   `json:"description"` // "" clears it
	Codes       *[]string `json:"codes"`       // replaces the role's permissions; [] removes them all
}

// ListRolesRequest lists the roles of a domain.
type ListRolesRequest struct {
	Domain  string `json:"domain"`  // your own domain when left out; a domain below yours otherwise
	Keyword string `json:"keyword"` // optional, matches the role name
}

type AssignRoleRequest struct {
	UserID uint64 `json:"user_id"`
	RoleID uint64 `json:"role_id"`
}

// GetAllRoles returns all roles
func GetAllRoles(c *fiber.Ctx) error {
	scope, ok := domainScope(c, true)
	if !ok {
		return nil
	}

	roles, err := domain.GetAllRoles(scope)
	if err != nil {
		utils.Logger.Errorw("Failed to fetch roles", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to retrieve roles",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Roles retrieved successfully",
		"data":    roles,
	})
}

func roleNameTakenResponse(c *fiber.Ctx) error {
	return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was saved", []models.PermissionIssue{
		{Path: "name", Message: "Role name already exists in this domain", Conflict: true},
	})
}

// validateRoleFields checks a role name and description, reporting every problem.
func validateRoleFields(dom *models.Domain, name *string, description *string) []models.PermissionIssue {
	var issues []models.PermissionIssue
	if name != nil {
		switch {
		case !models.ValidRoleName(*name):
			issues = append(issues, models.PermissionIssue{Path: "name", Message: "Role name is required (max 100 characters)"})
		case models.IsReservedRoleName(dom, *name):
			issues = append(issues, models.PermissionIssue{Path: "name", Message: "This role name is reserved"})
		}
	}
	if description != nil && !models.ValidRoleDescription(*description) {
		issues = append(issues, models.PermissionIssue{Path: "description", Message: "Description must be at most 255 characters"})
	}
	return issues
}

// CreateRole creates a new role, optionally giving it permissions in the same step
func CreateRole(c *fiber.Ctx) error {
	var req CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	domainName := req.Domain
	if domainName == "" {
		domainName = c.Query("domain") // older clients pass it in the URL
	}
	dom, ok := domainForRequest(c, domainName)
	if !ok {
		return nil
	}

	req.Name = strings.TrimSpace(req.Name)
	var description *string
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if trimmed != "" {
			description = &trimmed
		}
	}
	if issues := validateRoleFields(dom, &req.Name, description); len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was saved", issues)
	}

	if existing, _ := domain.GetRoleByName(dom.ID, req.Name); existing != nil {
		return roleNameTakenResponse(c)
	}

	role := models.Role{DomainID: dom.ID, Name: req.Name, Description: description}

	var err error
	if req.Codes != nil {
		var issues []models.PermissionIssue
		issues, err = domain.CreateRoleWithPermissions(&role, dom, req.Codes)
		if err == nil && len(issues) > 0 {
			return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Some codes are not available, nothing was saved", issues)
		}
	} else {
		err = domain.CreateRole(&role)
	}
	if err != nil {
		if existing, _ := domain.GetRoleByName(dom.ID, req.Name); existing != nil {
			return roleNameTakenResponse(c)
		}
		utils.Logger.Errorw("Failed to create role", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create role",
		})
	}

	// Reload so the response carries the domain name and permissions
	if created, err := domain.GetRoleByID(role.ID, dom.ID); err == nil && created != nil {
		role = *created
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "Role created successfully",
		"data":    role,
	})
}

// UpdateRole changes a role's name, description and/or permissions
func UpdateRole(c *fiber.Ctx) error {
	var req UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
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
	if req.Name == nil && req.Description == nil && req.Codes == nil {
		issues = append(issues, models.PermissionIssue{Path: "", Message: "Send a new name, description and/or codes"})
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", issues)
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

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
		if trimmed == role.Name {
			req.Name = nil // sending the current name is not a rename
		}
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		req.Description = &trimmed
	}
	issues = validateRoleFields(dom, req.Name, req.Description)
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", issues)
	}

	if req.Name != nil {
		// the shutterx superadmin role is what grants cross-domain power, so it keeps its name
		if models.IsReservedRoleName(dom, role.Name) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusForbidden,
				"message": "The superadmin role cannot be renamed",
			})
		}
		if existing, _ := domain.GetRoleByName(dom.ID, *req.Name); existing != nil && existing.ID != role.ID {
			return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was changed", []models.PermissionIssue{
				{Path: "name", Message: "Role name already exists in this domain", Conflict: true},
			})
		}
	}

	if req.Codes != nil {
		for i := range *req.Codes {
			(*req.Codes)[i] = strings.TrimSpace((*req.Codes)[i])
		}
	}

	issues, err = domain.UpdateRole(role, dom, req.Name, req.Description, req.Codes)
	if err != nil {
		utils.Logger.Errorw("Failed to update role", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to update role",
		})
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Some codes are not available, nothing was changed", issues)
	}

	updated, err := domain.GetRoleByID(role.ID, dom.ID)
	if err != nil || updated == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Role updated but could not be reloaded",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Role updated successfully",
		"data":    updated,
	})
}

// ListRoles lists the roles of the domain named in the body: your own, or one below yours
func ListRoles(c *fiber.Ctx) error {
	var req ListRolesRequest
	if len(strings.TrimSpace(string(c.Body()))) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusBadRequest,
				"message": "Invalid request payload",
			})
		}
	}

	dom, ok := domainForRead(c, req.Domain)
	if !ok {
		return nil
	}

	roles, err := domain.SearchRoles(dom.ID, strings.TrimSpace(req.Keyword))
	if err != nil {
		utils.Logger.Errorw("Failed to fetch roles", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to retrieve roles",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Roles retrieved successfully",
		"data":    roles,
	})
}

// AssignRole assigns a role to a user
func AssignRole(c *fiber.Ctx) error {
	var req AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	if req.UserID == 0 || req.RoleID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "user_id and role_id are required",
		})
	}

	scope, ok := domainScope(c, false)
	if !ok {
		return nil
	}

	// Both the user and the role must exist in the target domain
	targetUser, userErr := domain.GetUserByID(req.UserID, scope)
	targetRole, roleErr := domain.GetRoleByID(req.RoleID, scope)
	if userErr != nil || targetUser == nil || roleErr != nil || targetRole == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "User or role not found in this domain",
		})
	}

	if err := domain.AssignRoleToUser(req.UserID, req.RoleID); err != nil {
		utils.Logger.Errorw("Failed to assign role to user", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to assign role",
		})
	}

	user, _ := domain.GetUserByID(req.UserID, scope)
	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Role assigned successfully",
		"data":    user.ToResponse(),
	})
}

// GetRoleByID returns a single role by ID
func GetRoleByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid role ID",
		})
	}

	scope, ok := domainScope(c, true)
	if !ok {
		return nil
	}

	role, err := domain.GetRoleByID(id, scope)
	if err != nil || role == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Role not found",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Role retrieved successfully",
		"data":    role,
	})
}
