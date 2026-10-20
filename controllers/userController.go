package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type CreateUserRequest struct {
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"`
	Status   string  `json:"status"`
	RoleID   uint64  `json:"role_id"`
}

type UpdateUserRequest struct {
	Name   string  `json:"name"`
	Phone  *string `json:"phone"`
	Status string  `json:"status"`
}

// GetAllUsers handles paginated user listing with search keyword
func GetAllUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "10"))
	keyword := strings.TrimSpace(c.Query("keyword", ""))

	scope, ok := domainScope(c, true)
	if !ok {
		return nil
	}

	users, total, err := domain.GetAllUsers(page, pageSize, keyword, scope)
	if err != nil {
		utils.Logger.Errorw("Failed to fetch users", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to retrieve users",
		})
	}

	responseList := make([]models.UserResponse, 0, len(users))
	for _, u := range users {
		responseList = append(responseList, u.ToResponse())
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Users retrieved successfully",
		"data": fiber.Map{
			"users":     responseList,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetUserByID handles fetching a single user by ID
func GetUserByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid user ID",
		})
	}

	scope, ok := domainScope(c, true)
	if !ok {
		return nil
	}

	user, err := domain.GetUserByID(id, scope)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "User found",
		"data":    user.ToResponse(),
	})
}

// CreateUser creates a user from admin/management API
func CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Email == "" || req.Name == "" || len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Name, email, and password (min 8 chars) are required",
		})
	}

	scope, ok := domainScope(c, false)
	if !ok {
		return nil
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

	if existing, _ := domain.GetUserByEmail(scope, req.Email); existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Email is already registered",
		})
	}
	if existing, _ := domain.GetUserByUsername(scope, username); existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Username is already taken",
		})
	}

	// The role must belong to the same domain as the new user
	if req.RoleID > 0 {
		if role, err := domain.GetRoleByID(req.RoleID, scope); err != nil || role == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusBadRequest,
				"message": "Role not found in this domain",
			})
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Logger.Errorw("Failed to hash password", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to process user creation",
		})
	}

	status := "active"
	if req.Status == "inactive" || req.Status == "blocked" {
		status = req.Status
	}

	newUser := models.User{
		DomainID: scope,
		Username: username,
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: string(hashedPassword),
		Status:   status,
	}

	if err := domain.CreateUser(&newUser); err != nil {
		utils.Logger.Errorw("Failed to create user", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create user",
		})
	}

	if req.RoleID > 0 {
		_ = domain.AssignRoleToUser(newUser.ID, req.RoleID)
	}

	created, _ := domain.GetUserByID(newUser.ID, scope)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "User created successfully",
		"data":    created.ToResponse(),
	})
}

// UpdateUser updates user details
func UpdateUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid user ID",
		})
	}

	scope, ok := domainScope(c, false)
	if !ok {
		return nil
	}

	user, err := domain.GetUserByID(id, scope)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "User not found",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	if req.Name != "" {
		user.Name = strings.TrimSpace(req.Name)
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.Status == "active" || req.Status == "inactive" || req.Status == "blocked" {
		user.Status = req.Status
	}

	if err := domain.UpdateUser(user); err != nil {
		utils.Logger.Errorw("Failed to update user", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to update user",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "User updated successfully",
		"data":    user.ToResponse(),
	})
}

// DeleteUser deletes a user by ID
func DeleteUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid user ID",
		})
	}

	scope, ok := domainScope(c, false)
	if !ok {
		return nil
	}

	if err := domain.DeleteUser(id, scope); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusNotFound,
				"message": "User not found",
			})
		}
		utils.Logger.Errorw("Failed to delete user", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to delete user",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "User deleted successfully",
	})
}
