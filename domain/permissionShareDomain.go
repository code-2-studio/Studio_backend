package domain

import (
	"errors"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ShareResult describes what a share request changed (or would change, for a dry run).
type ShareResult struct {
	Added         int
	Removed       int
	RolesAffected int
	Tree          []models.FeatureGroupEnvelope // the owner's permissions the target has afterwards
}

// SharePermissions changes which of owner's permissions the target domain can see.
//
// Only permissions owned by owner can be shared, and replace/remove/clear only ever touch the shares
// that come from owner: shares the target received from other domains are left alone.
// Permissions taken away are also removed from the target's roles. With dryRun nothing is written.
// If the request does not match the owner's permissions, nothing is written and the issues are returned.
func SharePermissions(owner, target *models.Domain, mode string, groups []models.FeatureGroupItem, isArray, dryRun bool) (*ShareResult, []models.PermissionIssue, error) {
	if db.DB == nil {
		return nil, nil, errors.New("database not connected")
	}
	if target.IsRoot() {
		return nil, []models.PermissionIssue{{Path: "domain", Message: "The root domain already has every permission"}}, nil
	}
	if target.ID == owner.ID {
		return nil, []models.PermissionIssue{{Path: "domain", Message: "A domain already has its own permissions"}}, nil
	}

	var result *ShareResult
	var issues []models.PermissionIssue

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPermissionWrites(tx); err != nil {
			return err
		}

		var all []models.Permission
		if err := tx.Order("id ASC").Find(&all).Error; err != nil {
			return err
		}
		owned := make([]models.Permission, 0, len(all))
		for _, r := range all {
			if r.DomainID == owner.ID {
				owned = append(owned, r)
			}
		}

		current, err := sharedByOwnerIDs(tx, owner.ID, target.ID)
		if err != nil {
			return err
		}

		plan, planIssues := models.PlanPermissionShare(owned, current, mode, groups, isArray)
		if len(planIssues) > 0 {
			issues = planIssues
			return nil
		}

		rolesAffected := int64(0)
		if len(plan.Remove) > 0 {
			err := tx.Table("role_permissions AS rp").
				Joins("JOIN roles r ON r.id = rp.role_id").
				Where("r.domain_id = ? AND rp.permission_id IN ?", target.ID, plan.Remove).
				Distinct("rp.role_id").Count(&rolesAffected).Error
			if err != nil {
				return err
			}
		}

		if !dryRun {
			if len(plan.Remove) > 0 {
				if err := tx.Where("domain_id = ? AND permission_id IN ?", target.ID, plan.Remove).
					Delete(&models.DomainPermission{}).Error; err != nil {
					return err
				}
				if err := tx.Where("permission_id IN ? AND role_id IN (?)", plan.Remove,
					tx.Model(&models.Role{}).Select("id").Where("domain_id = ?", target.ID)).
					Delete(&models.RolePermission{}).Error; err != nil {
					return err
				}
			}
			if len(plan.Add) > 0 {
				links := make([]models.DomainPermission, 0, len(plan.Add))
				for _, id := range plan.Add {
					links = append(links, models.DomainPermission{DomainID: target.ID, PermissionID: id})
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error; err != nil {
					return err
				}
			}
		}

		next := make(map[uint64]bool, len(current)+len(plan.Add))
		for id := range current {
			next[id] = true
		}
		for _, id := range plan.Add {
			next[id] = true
		}
		for _, id := range plan.Remove {
			delete(next, id)
		}

		result = &ShareResult{
			Added:         len(plan.Add),
			Removed:       len(plan.Remove),
			RolesAffected: int(rolesAffected),
			Tree:          models.BuildPermissionTree(models.FilterPermissionRows(owned, next)),
		}
		return nil
	})
	return result, issues, err
}

// SetRolePermissions replaces a role's permissions with the given codes. Every code must be a
// permission-level code the role's domain can see (root: any; child: what it owns plus what was shared with it).
func SetRolePermissions(role *models.Role, dom *models.Domain, codes []string) ([]models.PermissionIssue, error) {
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
		return replaceRolePermissions(tx, role.ID, ids)
	})
	return issues, err
}
