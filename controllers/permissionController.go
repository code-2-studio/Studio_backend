package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

// ViewPermissionsRequest selects what to read. Both fields are optional.
type ViewPermissionsRequest struct {
	Domain     string `json:"domain"`      // root superadmin only: read another domain's view
	Code       string `json:"code"`        // return just the group containing this code
	Owned      bool   `json:"owned"`       // only permissions your domain created
	SharedWith string `json:"shared_with"` // only your own permissions already shared with this domain
}

// UpdatePermissionRequest renames a node and/or changes its code. Code says which node.
type UpdatePermissionRequest struct {
	Code    string  `json:"code"`
	Name    *string `json:"name"`
	NewCode *string `json:"new_code"`
}

func permissionIssuesResponse(c *fiber.Ctx, status int, message string, issues []models.PermissionIssue) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  false,
		"code":    status,
		"message": message,
		"errors":  issues,
	})
}

// permissionIssuesStatus is 409 when any problem is a clash with stored data, otherwise 422.
func permissionIssuesStatus(issues []models.PermissionIssue) int {
	for _, is := range issues {
		if is.Conflict {
			return fiber.StatusConflict
		}
	}
	return fiber.StatusUnprocessableEntity
}

func permissionServerError(c *fiber.Ctx, logMsg string, err error, userMsg string) error {
	utils.Logger.Errorw(logMsg, "error", err)
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"status":  false,
		"code":    http.StatusInternalServerError,
		"message": userMsg,
	})
}

// GetAllPermissions returns the permissions the caller's domain can see: the whole catalog for
// shutterx, only the shared permissions for a child domain.
func GetAllPermissions(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	tree, err := domain.GetPermissionTree(user.Domain, domain.PermissionViewOptions{})
	if err != nil {
		return permissionServerError(c, "Failed to fetch permissions", err, "Failed to retrieve permissions")
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Permissions retrieved successfully",
		"data":    tree,
	})
}

// ViewPermissions reads permissions with options in the body: a single code and/or, for the root
// superadmin, another domain's view.
func ViewPermissions(c *fiber.Ctx) error {
	var req ViewPermissionsRequest
	if len(strings.TrimSpace(string(c.Body()))) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusBadRequest,
				"message": "Invalid request payload",
			})
		}
	}

	dom, ok := domainForRequest(c, req.Domain)
	if !ok {
		return nil
	}

	opts := domain.PermissionViewOptions{OwnedOnly: req.Owned}
	if sharedWith := models.NormalizeDomainName(req.SharedWith); sharedWith != "" {
		if req.Owned {
			return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed", []models.PermissionIssue{
				{Path: "shared_with", Message: "Use either owned or shared_with, not both"},
			})
		}
		target, err := domain.GetDomainByName(sharedWith)
		if err != nil || target == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  false,
				"code":    http.StatusNotFound,
				"message": "Domain not found",
			})
		}
		opts.SharedWith = target
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		tree, err := domain.GetPermissionTree(dom, opts)
		if err != nil {
			return permissionServerError(c, "Failed to fetch permissions", err, "Failed to retrieve permissions")
		}
		return c.JSON(fiber.Map{
			"status":  true,
			"code":    http.StatusOK,
			"message": "Permissions retrieved successfully",
			"data":    tree,
		})
	}

	branch, err := domain.GetPermissionBranch(dom, opts, code)
	if err != nil {
		return permissionServerError(c, "Failed to fetch permission", err, "Failed to retrieve permission")
	}
	if branch == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Permission not found",
		})
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Permission found",
		"data":    branch,
	})
}

// CreatePermissions saves feature groups, features and permissions (each with its code). Any signed-in
// user may create them; they belong to the creator's domain. A group may be sent alone, or with any of
// its features and permissions.
func CreatePermissions(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	groups, isArray, issues := models.ParsePermissionPayload(c.Body())
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was saved", issues)
	}

	created, existing, conflicts, err := domain.CreatePermissions(user.DomainID, groups, isArray)
	if err != nil {
		return permissionServerError(c, "Failed to save permissions", err, "Failed to save permissions")
	}
	if len(conflicts) > 0 {
		return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was saved", conflicts)
	}

	data := fiber.Map{"created": created, "existing": existing}
	if isArray {
		envelopes := make([]models.FeatureGroupEnvelope, 0, len(groups))
		for _, g := range groups {
			envelopes = append(envelopes, models.FeatureGroupEnvelope{FeatureGroup: g})
		}
		data["featureGroups"] = envelopes
	} else {
		data["featureGroup"] = groups[0]
	}

	status := fiber.StatusCreated
	if created == 0 {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{
		"status":  true,
		"code":    status,
		"message": "Permissions saved successfully",
		"data":    data,
	})
}

// UpdatePermission renames a feature group, feature or permission and/or changes its code.
// Any signed-in user may do this for permissions their own domain created.
func UpdatePermission(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	var req UpdatePermissionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	req.Code = strings.TrimSpace(req.Code)
	var issues []models.PermissionIssue
	if req.Code == "" {
		issues = append(issues, models.PermissionIssue{Path: "code", Message: "Code of the item to update is required"})
	}
	if req.Name == nil && req.NewCode == nil {
		issues = append(issues, models.PermissionIssue{Path: "", Message: "Send a new name and/or a new_code"})
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
		if !models.ValidPermissionName(trimmed) {
			issues = append(issues, models.PermissionIssue{Path: "name", Message: "Name is required (max 100 characters)"})
		}
	}
	if req.NewCode != nil {
		trimmed := strings.TrimSpace(*req.NewCode)
		req.NewCode = &trimmed
		if !models.ValidPermissionCode(trimmed) {
			issues = append(issues, models.PermissionIssue{Path: "new_code", Message: "Code must be 2-32 letters or digits"})
		}
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", issues)
	}

	finalCode, err := domain.UpdatePermission(user.DomainID, req.Code, req.Name, req.NewCode)
	switch {
	case err == domain.ErrPermissionNotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Permission not found",
		})
	case err == domain.ErrPermissionNotOwned:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusForbidden,
			"message": "Forbidden: This permission belongs to another domain",
		})
	case err == domain.ErrPermissionCodeTaken:
		return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was changed", []models.PermissionIssue{
			{Path: "new_code", Message: "Code '" + *req.NewCode + "' already exists", Conflict: true},
		})
	case err == domain.ErrPermissionNameTaken:
		return permissionIssuesResponse(c, fiber.StatusConflict, "Conflicts found, nothing was changed", []models.PermissionIssue{
			{Path: "name", Message: "Name '" + *req.Name + "' is already used by another item at this level", Conflict: true},
		})
	case err != nil:
		return permissionServerError(c, "Failed to update permission", err, "Failed to update permission")
	}

	branch, err := domain.GetPermissionBranch(user.Domain, domain.PermissionViewOptions{}, finalCode)
	if err != nil || branch == nil {
		return permissionServerError(c, "Failed to reload updated permission", err, "Permission updated but could not be reloaded")
	}

	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": "Permission updated successfully",
		"data":    branch,
	})
}
