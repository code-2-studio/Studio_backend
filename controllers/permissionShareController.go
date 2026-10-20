package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"studio_backend/domain"
	"studio_backend/models"

	"github.com/gofiber/fiber/v2"
)

// SharePermissionsRequest changes which permissions a child domain can see.
// Permissions is the same JSON shape the catalog returns: one {"featureGroup": ...} or an array of them.
type SharePermissionsRequest struct {
	Domain      string          `json:"domain"`
	Mode        string          `json:"mode"`    // replace (default), add, remove, clear
	DryRun      bool            `json:"dry_run"` // preview only, change nothing
	Permissions json.RawMessage `json:"permissions"`
}

// SharePermissions shares many of the caller's domain's own permissions with one other domain in a
// single request. Any signed-in user may do this; no role is checked.
func SharePermissions(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Domain == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized: Authentication required",
		})
	}

	var req SharePermissionsRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusBadRequest,
			"message": "Invalid request payload",
		})
	}

	domainName := models.NormalizeDomainName(req.Domain)
	if domainName == "" {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", []models.PermissionIssue{
			{Path: "domain", Message: "Domain is required"},
		})
	}

	dom, err := domain.GetDomainByName(domainName)
	if err != nil || dom == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  false,
			"code":    http.StatusNotFound,
			"message": "Domain not found",
		})
	}
	if dom.IsRoot() {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", []models.PermissionIssue{
			{Path: "domain", Message: "The root domain already has every permission"},
		})
	}
	if dom.ID == user.DomainID {
		return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", []models.PermissionIssue{
			{Path: "domain", Message: "A domain already has its own permissions"},
		})
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = models.ShareModeReplace
	}

	var groups []models.FeatureGroupItem
	isArray := false
	if mode != models.ShareModeClear {
		var parseIssues []models.PermissionIssue
		if len(strings.TrimSpace(string(req.Permissions))) == 0 || string(req.Permissions) == "null" {
			parseIssues = []models.PermissionIssue{{Path: "permissions", Message: "Send at least one permission"}}
		} else {
			groups, isArray, parseIssues = models.ParsePermissionPayload(req.Permissions)
			for i := range parseIssues {
				if parseIssues[i].Path == "" {
					parseIssues[i].Path = "permissions"
				} else {
					parseIssues[i].Path = "permissions" + prefixIfArray(parseIssues[i].Path)
				}
			}
		}
		if len(parseIssues) > 0 {
			return permissionIssuesResponse(c, fiber.StatusUnprocessableEntity, "Validation failed, nothing was changed", parseIssues)
		}
	}

	result, issues, err := domain.SharePermissions(user.Domain, dom, mode, groups, isArray, req.DryRun)
	if err != nil {
		return permissionServerError(c, "Failed to share permissions", err, "Failed to share permissions")
	}
	if len(issues) > 0 {
		return permissionIssuesResponse(c, permissionIssuesStatus(issues), "Request does not match your domain's permissions, nothing was changed", issues)
	}

	message := "Permissions shared successfully"
	if req.DryRun {
		message = "Dry run, nothing was changed"
	}
	return c.JSON(fiber.Map{
		"status":  true,
		"code":    http.StatusOK,
		"message": message,
		"data": fiber.Map{
			"domain":         dom.Name,
			"mode":           mode,
			"dry_run":        req.DryRun,
			"added":          result.Added,
			"removed":        result.Removed,
			"roles_affected": result.RolesAffected,
			"permissions":    result.Tree,
		},
	})
}

// prefixIfArray makes a parser path ("featureGroup.x" or "[1].featureGroup.x") relative to the
// "permissions" field of the share request.
func prefixIfArray(path string) string {
	if strings.HasPrefix(path, "[") {
		return path
	}
	return "[0]." + path
}
