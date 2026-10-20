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

type CreateRoleRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
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

// CreateRole creates a new role
func CreateRole(c *fiber.Ctx) error {
	var req CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Role name is required",
		})
	}

	scope, ok := domainScope(c, false)
	if !ok {
		return nil
	}

	role := models.Role{
		DomainID:    scope,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := domain.CreateRole(&role); err != nil {
		if existing, _ := domain.GetRoleByName(scope, req.Name); existing != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusConflict,
				"message": "Role name already exists in this domain",
			})
		}
		utils.Logger.Errorw("Failed to create role", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create role",
		})
	}

	// Reload so the response carries the domain name
	if created, err := domain.GetRoleByID(role.ID, scope); err == nil && created != nil {
		role = *created
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "Role created successfully",
		"data":    role,
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
