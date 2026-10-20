package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type CheckStudioDomainRequest struct {
	Domain string `json:"domain"`
}

// OnboardStudio creates a studio: a new domain below the caller's domain, with its profile, an Admin
// role, and an admin user (username@domain) holding that role. Any signed-in user may do this.
func OnboardStudio(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	var req models.StudioRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	in, issues := models.ParseStudioRequest(req)
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was created", issues)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return permissionServerError(c, "Failed to hash studio admin password", err, "Failed to create studio")
	}

	studio, conflicts, err := domain.OnboardStudio(user.Domain, in, string(hash))
	if err != nil {
		return permissionServerError(c, "Failed to onboard studio", err, "Failed to create studio")
	}
	if len(conflicts) > 0 {
		return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was created", conflicts)
	}

	utils.Logger.Infow("Studio onboarded", "domain", studio.Studio.Domain, "parent", user.Domain.Name, "created_by", user.ID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "Studio created successfully",
		"data":    studio,
	})
}

// CheckStudioDomain says whether a studio domain name can still be used, for live checks in the form.
func CheckStudioDomain(c *fiber.Ctx) error {
	var req CheckStudioDomainRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	name := models.NormalizeDomainName(req.Domain)
	if name == "" || !models.ValidDomainName(name) {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed", []models.FieldIssue{
			{Path: "domain", Message: "Domain must be 3-50 characters: lowercase letters, digits and hyphens only"},
		})
	}

	reason := ""
	available := true
	if models.IsReservedDomainName(name) {
		available, reason = false, "reserved"
	} else {
		free, err := domain.DomainNameAvailable(name)
		if err != nil {
			return permissionServerError(c, "Failed to check domain", err, "Failed to check domain")
		}
		if !free {
			available, reason = false, "taken"
		}
	}

	data := fiber.Map{"domain": name, "available": available, "admin_login": "admin@" + name}
	if reason != "" {
		data["reason"] = reason
	}
	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Domain checked",
		"data":    data,
	})
}

// canViewStudio is true for shutterx, for your own domain, and for any domain below yours.
func canViewStudio(user *models.User, id uint64) bool {
	if user.Domain.IsRoot() || id == user.DomainID {
		return true
	}
	within, err := domain.IsDomainWithin(user.DomainID, id)
	return err == nil && within
}

func studioNotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"status":  false,
		"code":    http.StatusNotFound,
		"message": "Studio not found",
	})
}

// GetStudio returns a studio's domain with the owner and studio details saved at onboarding.
// You can read your own studio and the studios below it; shutterx can read any.
func GetStudio(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid studio ID",
		})
	}
	if !canViewStudio(user, id) {
		return studioNotFound(c)
	}

	studio, err := domain.GetStudio(id)
	if err != nil {
		return permissionServerError(c, "Failed to fetch studio", err, "Failed to retrieve studio")
	}
	if studio == nil {
		return studioNotFound(c)
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studio retrieved successfully",
		"data":    studio,
	})
}

// UpdateStudio replaces a studio's owner and studio details. The users of the studio's own domain and
// the shutterx superadmin may do this. The domain name and admin user are not changed.
func UpdateStudio(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid studio ID",
		})
	}
	if !canViewStudio(user, id) {
		return studioNotFound(c)
	}
	if id != user.DomainID && !user.IsRootSuperAdmin() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Forbidden: Only the studio's own users or the shutterx superadmin can edit it",
		})
	}

	dom, err := domain.GetDomainByID(id)
	if err != nil || dom == nil {
		return studioNotFound(c)
	}
	if dom.IsRoot() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "The root domain cannot be modified",
		})
	}

	var req models.StudioProfileRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	in, issues := models.ParseStudioProfileRequest(req, dom.Name)
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", issues)
	}

	studio, err := domain.UpdateStudio(id, in)
	if err != nil {
		return permissionServerError(c, "Failed to update studio", err, "Failed to update studio")
	}
	if studio == nil {
		return studioNotFound(c)
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studio updated successfully",
		"data":    studio,
	})
}
