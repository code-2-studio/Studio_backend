package controllers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

var studioSlugRegex = regexp.MustCompile(`^[a-z0-9-]{3,50}$`)

type OwnerPayload struct {
	FullName      string `json:"full_name"`
	Mobile        string `json:"mobile"`
	BusinessEmail string `json:"business_email"`
}

type StudioPayload struct {
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Handle    string  `json:"handle"`
	Domain    string  `json:"domain"` // fallback
	Address   string  `json:"address"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	Pincode   string  `json:"pincode"`
	WhatsApp  *string `json:"whatsapp"`
	GSTIN     *string `json:"gstin"`
	Website   *string `json:"website"`
	Instagram *string `json:"instagram"`
	Facebook  *string `json:"facebook"`
	YouTube   *string `json:"youtube"`
}

type AdminPayload struct {
	FullName string `json:"full_name"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateStudioRequest struct {
	Owner  OwnerPayload  `json:"owner"`
	Studio StudioPayload `json:"studio"`
	Admin  AdminPayload  `json:"admin"`

	// Flat field fallbacks for convenient Postman calls
	StudioName    string `json:"studio_name"`
	Slug          string `json:"slug"`
	Handle        string `json:"handle"`
	DomainSlug    string `json:"domain"`
	OwnerFullName string `json:"owner_name"`
	OwnerPhone    string `json:"owner_mobile"`
	OwnerEmail    string `json:"owner_email"`
	Address       string `json:"address"`
	City          string `json:"city"`
	State         string `json:"state"`
	Pincode       string `json:"pincode"`
	Password      string `json:"password"`
}

type CheckStudioSlugRequest struct {
	Slug   string `json:"slug"`
	Handle string `json:"handle"`
	Domain string `json:"domain"`
}

// CheckStudioSlug checks if a studio slug / handle is available for onboarding
func CheckStudioSlug(c *fiber.Ctx) error {
	var req CheckStudioSlugRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = strings.TrimSpace(req.Handle)
	}
	if slug == "" {
		slug = strings.TrimSpace(req.Domain)
	}

	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Studio slug/handle parameter is required",
		})
	}

	clean := strings.ToLower(slug)
	if !studioSlugRegex.MatchString(clean) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Studio slug must be 3-50 lowercase alphanumeric characters or hyphens",
		})
	}

	available, reason, err := domain.CheckDomainAvailable(clean)
	if err != nil {
		utils.Logger.Errorw("Failed to check studio slug availability", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to check studio availability",
		})
	}

	response := fiber.Map{
		"slug":        clean,
		"handle":      clean,
		"available":   available,
		"admin_login": "admin@" + clean,
	}
	if !available {
		response["reason"] = reason
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studio availability checked",
		"data":    response,
	})
}

// CheckStudioDomain is an alias for CheckStudioSlug
func CheckStudioDomain(c *fiber.Ctx) error {
	return CheckStudioSlug(c)
}

// CreateStudio onboards a new studio with profile, admin role, and admin user
func CreateStudio(c *fiber.Ctx) error {
	var req CreateStudioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	// Consolidate nested & flat inputs
	studioName := strings.TrimSpace(req.Studio.Name)
	if studioName == "" {
		studioName = strings.TrimSpace(req.StudioName)
	}

	slug := strings.ToLower(strings.TrimSpace(req.Studio.Slug))
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(req.Studio.Handle))
	}
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(req.Studio.Domain))
	}
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(req.Slug))
	}
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(req.Handle))
	}
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(req.DomainSlug))
	}

	ownerName := strings.TrimSpace(req.Owner.FullName)
	if ownerName == "" {
		ownerName = strings.TrimSpace(req.OwnerFullName)
	}

	ownerPhone := strings.TrimSpace(req.Owner.Mobile)
	if ownerPhone == "" {
		ownerPhone = strings.TrimSpace(req.OwnerPhone)
	}

	ownerEmail := strings.ToLower(strings.TrimSpace(req.Owner.BusinessEmail))
	if ownerEmail == "" {
		ownerEmail = strings.ToLower(strings.TrimSpace(req.OwnerEmail))
	}

	address := strings.TrimSpace(req.Studio.Address)
	if address == "" {
		address = strings.TrimSpace(req.Address)
	}

	city := strings.TrimSpace(req.Studio.City)
	if city == "" {
		city = strings.TrimSpace(req.City)
	}

	state := strings.TrimSpace(req.Studio.State)
	if state == "" {
		state = strings.TrimSpace(req.State)
	}

	pincode := strings.TrimSpace(req.Studio.Pincode)
	if pincode == "" {
		pincode = strings.TrimSpace(req.Pincode)
	}

	adminPassword := req.Admin.Password
	if adminPassword == "" {
		adminPassword = req.Password
	}

	adminUsername := strings.ToLower(strings.TrimSpace(req.Admin.Username))
	if adminUsername == "" {
		adminUsername = "admin"
	}

	adminEmail := strings.ToLower(strings.TrimSpace(req.Admin.Email))
	if adminEmail == "" {
		adminEmail = ownerEmail
	}

	adminName := strings.TrimSpace(req.Admin.FullName)
	if adminName == "" {
		adminName = ownerName
	}

	adminMobile := strings.TrimSpace(req.Admin.Mobile)
	if adminMobile == "" {
		adminMobile = ownerPhone
	}

	// Validations
	if studioName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Studio/business name is required",
		})
	}

	if !studioSlugRegex.MatchString(slug) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Studio slug/handle must be 3-50 lowercase alphanumeric characters or hyphens",
		})
	}

	if ownerName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Owner full name is required",
		})
	}

	if ownerEmail == "" || !strings.Contains(ownerEmail, "@") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Valid owner business email is required",
		})
	}

	if len(adminPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Admin password must be at least 8 characters long",
		})
	}

	// Check studio availability
	available, reason, err := domain.CheckDomainAvailable(slug)
	if err != nil {
		utils.Logger.Errorw("Studio slug check error", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Error validating studio availability",
		})
	}
	if !available {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "Studio handle '" + slug + "' is " + reason,
		})
	}

	// Check if admin user email is already in use
	existingUser, _ := domain.GetUserByEmail(adminEmail)
	if existingUser != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusConflict,
			"message": "User with email '" + adminEmail + "' already exists",
		})
	}

	// Build Models
	studioModel := models.Studio{
		Name:          slug,
		DisplayName:   &studioName,
		OwnerName:     ownerName,
		OwnerMobile:   ownerPhone,
		BusinessEmail: ownerEmail,
		Address:       address,
		City:          city,
		State:         state,
		Pincode:       pincode,
		WhatsApp:      req.Studio.WhatsApp,
		GSTIN:         req.Studio.GSTIN,
		Website:       req.Studio.Website,
		Instagram:     req.Studio.Instagram,
		Facebook:      req.Studio.Facebook,
		YouTube:       req.Studio.YouTube,
	}

	adminModel := models.User{
		Username: &adminUsername,
		Name:     adminName,
		Email:    adminEmail,
		Phone:    &adminMobile,
	}

	createdStudio, createdAdmin, err := domain.CreateStudio(&studioModel, &adminModel, adminPassword)
	if err != nil {
		utils.Logger.Errorw("Failed to onboard studio", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to onboard studio: " + err.Error(),
		})
	}

	loginHandle := adminUsername + "@" + createdStudio.Name

	studioDisplayName := ""
	if createdStudio.DisplayName != nil {
		studioDisplayName = *createdStudio.DisplayName
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusCreated,
		"message": "Studio onboarded successfully",
		"data": fiber.Map{
			"studio": fiber.Map{
				"id":         createdStudio.ID,
				"name":       studioDisplayName,
				"slug":       createdStudio.Name,
				"handle":     createdStudio.Name,
				"status":     createdStudio.Status,
				"address":    createdStudio.Address,
				"city":       createdStudio.City,
				"state":      createdStudio.State,
				"pincode":    createdStudio.Pincode,
				"whatsapp":   createdStudio.WhatsApp,
				"gstin":      createdStudio.GSTIN,
				"website":    createdStudio.Website,
				"instagram":  createdStudio.Instagram,
				"facebook":   createdStudio.Facebook,
				"youtube":    createdStudio.YouTube,
				"created_at": createdStudio.CreatedAt,
				"updated_at": createdStudio.UpdatedAt,
			},
			"owner": fiber.Map{
				"full_name":      createdStudio.OwnerName,
				"mobile":         createdStudio.OwnerMobile,
				"business_email": createdStudio.BusinessEmail,
			},
			"admin": fiber.Map{
				"user_id":  createdAdmin.ID,
				"login":    loginHandle,
				"username": adminUsername,
				"email":    createdAdmin.Email,
				"role":     "Admin",
			},
		},
	})
}

// GetStudioByID returns the studio details by studio ID
func GetStudioByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid studio ID",
		})
	}

	s, err := domain.GetStudioByID(id)
	if err != nil || s == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Studio not found",
		})
	}

	studioName := ""
	if s.DisplayName != nil {
		studioName = *s.DisplayName
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studio details retrieved successfully",
		"data": fiber.Map{
			"studio": fiber.Map{
				"id":         s.ID,
				"name":       studioName,
				"slug":       s.Name,
				"handle":     s.Name,
				"status":     s.Status,
				"address":    s.Address,
				"city":       s.City,
				"state":      s.State,
				"pincode":    s.Pincode,
				"whatsapp":   s.WhatsApp,
				"gstin":      s.GSTIN,
				"website":    s.Website,
				"instagram":  s.Instagram,
				"facebook":   s.Facebook,
				"youtube":    s.YouTube,
				"created_at": s.CreatedAt,
				"updated_at": s.UpdatedAt,
			},
			"owner": fiber.Map{
				"full_name":      s.OwnerName,
				"mobile":         s.OwnerMobile,
				"business_email": s.BusinessEmail,
			},
		},
	})
}

// UpdateStudio updates studio details
func UpdateStudio(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid studio ID",
		})
	}

	var req CreateStudioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	studioName := strings.TrimSpace(req.Studio.Name)
	if studioName == "" {
		studioName = strings.TrimSpace(req.StudioName)
	}

	updates := models.Studio{
		OwnerName:     strings.TrimSpace(req.Owner.FullName),
		OwnerMobile:   strings.TrimSpace(req.Owner.Mobile),
		BusinessEmail: strings.TrimSpace(req.Owner.BusinessEmail),
		Address:       strings.TrimSpace(req.Studio.Address),
		City:          strings.TrimSpace(req.Studio.City),
		State:         strings.TrimSpace(req.Studio.State),
		Pincode:       strings.TrimSpace(req.Studio.Pincode),
		WhatsApp:      req.Studio.WhatsApp,
		GSTIN:         req.Studio.GSTIN,
		Website:       req.Studio.Website,
		Instagram:     req.Studio.Instagram,
		Facebook:      req.Studio.Facebook,
		YouTube:       req.Studio.YouTube,
	}

	if studioName != "" {
		updates.DisplayName = &studioName
	}

	s, err := domain.UpdateStudio(id, &updates)
	if err != nil {
		utils.Logger.Errorw("Failed to update studio", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to update studio details",
		})
	}

	updatedStudioName := ""
	if s.DisplayName != nil {
		updatedStudioName = *s.DisplayName
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studio details updated successfully",
		"data": fiber.Map{
			"studio": fiber.Map{
				"id":         s.ID,
				"name":       updatedStudioName,
				"slug":       s.Name,
				"handle":     s.Name,
				"status":     s.Status,
				"address":    s.Address,
				"city":       s.City,
				"state":      s.State,
				"pincode":    s.Pincode,
				"whatsapp":   s.WhatsApp,
				"gstin":      s.GSTIN,
				"website":    s.Website,
				"instagram":  s.Instagram,
				"facebook":   s.Facebook,
				"youtube":    s.YouTube,
				"created_at": s.CreatedAt,
				"updated_at": s.UpdatedAt,
			},
			"owner": fiber.Map{
				"full_name":      s.OwnerName,
				"mobile":         s.OwnerMobile,
				"business_email": s.BusinessEmail,
			},
		},
	})
}

// GetAllStudios returns paginated list of studios
func GetAllStudios(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "10"))
	keyword := strings.TrimSpace(c.Query("keyword", ""))

	domainsList, total, err := domain.GetAllDomains(page, pageSize, keyword)
	if err != nil {
		utils.Logger.Errorw("Failed to fetch studios", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusInternalServerError,
			"message": "Failed to retrieve studios",
		})
	}

	type studioItem struct {
		ID        uint64 `json:"id"`
		Name      string `json:"name"`
		Slug      string `json:"slug"`
		Handle    string `json:"handle"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}

	studios := make([]studioItem, 0, len(domainsList))
	for _, d := range domainsList {
		name := ""
		if d.DisplayName != nil {
			name = *d.DisplayName
		}
		studios = append(studios, studioItem{
			ID:        d.ID,
			Name:      name,
			Slug:      d.Name,
			Handle:    d.Name,
			Status:    d.Status,
			CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Studios retrieved successfully",
		"data": fiber.Map{
			"studios":   studios,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetAllDomains is an alias for GetAllStudios for backwards compatibility
func GetAllDomains(c *fiber.Ctx) error {
	return GetAllStudios(c)
}


