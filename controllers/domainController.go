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

type CreateDomainRequest struct {
	Name        string  `json:"name"`
	DisplayName *string `json:"display_name"`
}

type UpdateDomainRequest struct {
	DisplayName *string `json:"display_name"`
	Status      string  `json:"status"`
}

// GetAllDomains handles paginated domain listing. shutterx sees every domain; any other domain
// sees the domains below it (the studios it onboarded, and theirs).
func GetAllDomains(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}
	ancestorID := uint64(0)
	if !user.Domain.IsRoot() {
		ancestorID = user.DomainID
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "10"))
	keyword := strings.TrimSpace(c.Query("keyword", ""))

	domains, total, err := domain.GetAllDomains(page, pageSize, keyword, ancestorID)
	if err != nil {
		utils.Logger.Errorw("Failed to fetch domains", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to retrieve domains",
		})
	}

	responseList := make([]models.DomainResponse, 0, len(domains))
	for i := range domains {
		responseList = append(responseList, domains[i].ToResponse())
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Domains retrieved successfully",
		"data": fiber.Map{
			"domains":   responseList,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetDomainByID returns a single domain: any domain for shutterx, otherwise your own or one below it
func GetDomainByID(c *fiber.Ctx) error {
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
			"message": "Invalid domain ID",
		})
	}

	if !user.Domain.IsRoot() && id != user.DomainID {
		if within, err := domain.IsDomainWithin(user.DomainID, id); err != nil || !within {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusNotFound,
				"message": "Domain not found",
			})
		}
	}

	dom, err := domain.GetDomainByID(id)
	if err != nil || dom == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Domain not found",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Domain found",
		"data":    dom.ToResponse(),
	})
}

// CreateDomain creates a new child domain under the root (shutterx) domain
func CreateDomain(c *fiber.Ctx) error {
	var req CreateDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	name := models.NormalizeDomainName(req.Name)
	if !models.ValidDomainName(name) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Domain name must be 3-50 characters: lowercase letters, digits and hyphens only",
		})
	}

	if existing, _ := domain.GetDomainByName(name); existing != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Domain name is already taken",
		})
	}

	root, err := domain.GetRootDomain()
	if err != nil {
		utils.Logger.Errorw("Failed to load root domain", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create domain",
		})
	}

	newDomain := models.Domain{
		Name:        name,
		DisplayName: req.DisplayName,
		ParentID:    &root.ID,
		Status:      models.DomainStatusActive,
	}

	if err := domain.CreateDomain(&newDomain); err != nil {
		// A concurrent request may have taken the name between the check and the insert
		if existing, _ := domain.GetDomainByName(name); existing != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusConflict,
				"message": "Domain name is already taken",
			})
		}
		utils.Logger.Errorw("Failed to create domain", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to create domain",
		})
	}

	newDomain.Parent = root

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "Domain created successfully",
		"data":    newDomain.ToResponse(),
	})
}

// UpdateDomain changes a child domain's display name or status. The root domain is immutable.
func UpdateDomain(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid domain ID",
		})
	}

	dom, err := domain.GetDomainByID(id)
	if err != nil || dom == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Domain not found",
		})
	}

	if dom.IsRoot() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "The root domain cannot be modified",
		})
	}

	var req UpdateDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	if req.DisplayName != nil {
		dom.DisplayName = req.DisplayName
	}
	if req.Status == models.DomainStatusActive || req.Status == models.DomainStatusInactive {
		dom.Status = req.Status
	}

	if err := domain.UpdateDomain(dom); err != nil {
		utils.Logger.Errorw("Failed to update domain", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to update domain",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Domain updated successfully",
		"data":    dom.ToResponse(),
	})
}
