package domain

import (
	"errors"
	"strconv"
	"strings"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
)

// resolveRoleCodes turns permission codes into permission ids. Every code must be a permission-level
// code the domain can see (root: any; child: what it owns plus what was shared with it). Problems are
// returned together, in which case ids is nil.
func resolveRoleCodes(tx *gorm.DB, dom *models.Domain, codes []string) ([]uint64, []models.PermissionIssue, error) {
	rows, err := visiblePermissionRows(tx, dom)
	if err != nil {
		return nil, nil, err
	}
	byCode := make(map[string]*models.Permission, len(rows))
	for i := range rows {
		byCode[rows[i].Code] = &rows[i]
	}

	var issues []models.PermissionIssue
	ids := make([]uint64, 0, len(codes))
	seen := map[uint64]bool{}
	for i, code := range codes {
		node := byCode[code]
		path := "codes[" + strconv.Itoa(i) + "]"
		switch {
		case node == nil:
			issues = append(issues, models.PermissionIssue{Path: path, Message: "Code '" + code + "' is not available to this domain"})
		case node.Level != models.PermissionLevelPermission:
			issues = append(issues, models.PermissionIssue{Path: path, Message: "Code '" + code + "' is a " + node.Level + "; only permission codes can be given to a role"})
		case !seen[node.ID]:
			seen[node.ID] = true
			ids = append(ids, node.ID)
		}
	}
	if len(issues) > 0 {
		return nil, issues, nil
	}
	return ids, nil, nil
}

// replaceRolePermissions makes ids the complete permission set of the role.
func replaceRolePermissions(tx *gorm.DB, roleID uint64, ids []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	links := make([]models.RolePermission, 0, len(ids))
	for _, id := range ids {
		links = append(links, models.RolePermission{RoleID: roleID, PermissionID: id})
	}
	return tx.Create(&links).Error
}

// SearchRoles lists a domain's roles with their permissions, optionally filtered by a name keyword.
func SearchRoles(domainID uint64, keyword string) ([]models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	query := db.DB.Where("domain_id = ?", domainID)
	if keyword != "" {
		query = query.Where("lower(name) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}

	var roles []models.Role
	err := query.Preload("Domain").Preload("Permissions").Order("id ASC").Find(&roles).Error
	return roles, err
}

// CreateRoleWithPermissions creates a role and, in the same transaction, gives it the permissions for
// codes. If any code is not available nothing is created and the issues are returned.
func CreateRoleWithPermissions(role *models.Role, dom *models.Domain, codes []string) ([]models.PermissionIssue, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var issues []models.PermissionIssue
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPermissionWrites(tx); err != nil {
			return err
		}

		ids, resolveIssues, err := resolveRoleCodes(tx, dom, codes)
		if err != nil {
			return err
		}
		if len(resolveIssues) > 0 {
			issues = resolveIssues
			return nil
		}

		if err := tx.Create(role).Error; err != nil {
			return err
		}
		return replaceRolePermissions(tx, role.ID, ids)
	})
	return issues, err
}

// UpdateRole changes a role's name, description and/or permissions in one transaction. A nil argument
// means "leave unchanged"; an empty description clears it; an empty codes list removes every permission.
// If any code is not available nothing is changed and the issues are returned.
func UpdateRole(role *models.Role, dom *models.Domain, name, description *string, codes *[]string) ([]models.PermissionIssue, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var issues []models.PermissionIssue
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPermissionWrites(tx); err != nil {
			return err
		}

		var ids []uint64
		if codes != nil {
			resolved, resolveIssues, err := resolveRoleCodes(tx, dom, *codes)
			if err != nil {
				return err
			}
			if len(resolveIssues) > 0 {
				issues = resolveIssues
				return nil
			}
			ids = resolved
		}

		if name != nil || description != nil {
			if name != nil {
				role.Name = *name
			}
			if description != nil {
				if *description == "" {
					role.Description = nil
				} else {
					role.Description = description
				}
			}
			if err := tx.Omit("Domain", "Permissions").Save(role).Error; err != nil {
				return err
			}
		}

		if codes != nil {
			return replaceRolePermissions(tx, role.ID, ids)
		}
		return nil
	})
	return issues, err
}
