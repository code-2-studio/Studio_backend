package domain

import (
	"errors"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
)

var (
	ErrPermissionNotFound  = errors.New("permission not found")
	ErrPermissionNotOwned  = errors.New("permission belongs to another domain")
	ErrPermissionCodeTaken = errors.New("permission code already exists")
	ErrPermissionNameTaken = errors.New("a sibling with this name already exists")
)

// permissionWriteLock serialises catalog writes so concurrent requests cannot both pass the
// conflict checks and then collide on insert.
const permissionWriteLock = 7301001

func lockPermissionWrites(tx *gorm.DB) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(?)", permissionWriteLock).Error
}

// visiblePermissionRows returns the catalog rows a domain may see: everything for the root domain,
// otherwise the permissions it owns plus the ones shared with it, with the groups/features above them.
func visiblePermissionRows(tx *gorm.DB, dom *models.Domain) ([]models.Permission, error) {
	var rows []models.Permission
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if dom.IsRoot() {
		return rows, nil
	}

	allowed, err := sharedPermissionIDs(tx, dom.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.DomainID == dom.ID && r.Level == models.PermissionLevelPermission {
			allowed[r.ID] = true
		}
	}
	return models.FilterPermissionRows(rows, allowed), nil
}

func sharedPermissionIDs(tx *gorm.DB, domainID uint64) (map[uint64]bool, error) {
	var ids []uint64
	if err := tx.Model(&models.DomainPermission{}).Where("domain_id = ?", domainID).Pluck("permission_id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// PermissionViewOptions narrows what a domain's view shows.
type PermissionViewOptions struct {
	OwnedOnly  bool           // only permissions this domain created
	SharedWith *models.Domain // only this domain's own permissions that are shared with the given domain
}

// ownedPermissionIDs returns the ids of the permission-level nodes a domain created.
func ownedPermissionIDs(rows []models.Permission, domainID uint64) map[uint64]bool {
	set := map[uint64]bool{}
	for _, r := range rows {
		if r.DomainID == domainID && r.Level == models.PermissionLevelPermission {
			set[r.ID] = true
		}
	}
	return set
}

// sharedByOwnerIDs returns the ids of permissions owned by ownerID that are shared with targetID.
func sharedByOwnerIDs(tx *gorm.DB, ownerID, targetID uint64) (map[uint64]bool, error) {
	var ids []uint64
	err := tx.Table("domain_permissions AS dp").
		Joins("JOIN permissions p ON p.id = dp.permission_id").
		Where("dp.domain_id = ? AND p.domain_id = ?", targetID, ownerID).
		Pluck("dp.permission_id", &ids).Error
	if err != nil {
		return nil, err
	}
	set := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// viewRows returns the rows for a domain's view, narrowed by the options.
func viewRows(tx *gorm.DB, dom *models.Domain, opts PermissionViewOptions) ([]models.Permission, error) {
	if !opts.OwnedOnly && opts.SharedWith == nil {
		return visiblePermissionRows(tx, dom)
	}

	var rows []models.Permission
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if opts.SharedWith != nil {
		shared, err := sharedByOwnerIDs(tx, dom.ID, opts.SharedWith.ID)
		if err != nil {
			return nil, err
		}
		return models.FilterPermissionRows(rows, shared), nil
	}
	return models.FilterPermissionRows(rows, ownedPermissionIDs(rows, dom.ID)), nil
}

// GetPermissionTree returns the permissions a domain can see, in the API's JSON shape.
func GetPermissionTree(dom *models.Domain, opts PermissionViewOptions) ([]models.FeatureGroupEnvelope, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := viewRows(db.DB, dom, opts)
	if err != nil {
		return nil, err
	}
	return models.BuildPermissionTree(rows), nil
}

// GetPermissionBranch returns the group that contains the given code, as the domain sees it.
// It returns nil when the code is unknown or not visible to the domain.
func GetPermissionBranch(dom *models.Domain, opts PermissionViewOptions, code string) (*models.FeatureGroupEnvelope, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := viewRows(db.DB, dom, opts)
	if err != nil {
		return nil, err
	}
	return models.BuildPermissionBranch(rows, code), nil
}

// CreatePermissions saves a validated request. New nodes belong to ownerDomainID, the creator's domain.
// If the request conflicts with the stored catalog nothing is written and the conflicts are returned.
func CreatePermissions(ownerDomainID uint64, groups []models.FeatureGroupItem, isArray bool) (created, existing int, issues []models.PermissionIssue, err error) {
	if db.DB == nil {
		return 0, 0, nil, errors.New("database not connected")
	}

	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPermissionWrites(tx); err != nil {
			return err
		}

		var rows []models.Permission
		if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}

		plan, existingCount, planIssues := models.PlanPermissionSave(rows, ownerDomainID, groups, isArray)
		existing = existingCount
		if len(planIssues) > 0 {
			issues = planIssues
			return nil
		}

		if err := insertPlannedPermissions(tx, ownerDomainID, rows, plan); err != nil {
			return err
		}
		created = len(plan)
		return nil
	})
	return created, existing, issues, err
}

// insertPlannedPermissions creates the planned nodes for ownerDomainID, parents before their children.
// existing are the rows already stored, used to find the parent of a node whose parent is not new.
func insertPlannedPermissions(tx *gorm.DB, ownerDomainID uint64, existing []models.Permission, plan []models.PlannedPermission) error {
	idByCode := make(map[string]uint64, len(existing)+len(plan))
	for _, r := range existing {
		idByCode[r.Code] = r.ID
	}

	for _, p := range plan {
		node := models.Permission{
			DomainID: ownerDomainID,
			Level:    p.Level,
			Name:     p.Name,
			Code:     p.Code,
		}
		if p.ParentCode != "" {
			parentID := idByCode[p.ParentCode]
			node.ParentID = &parentID
		}
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
		idByCode[p.Code] = node.ID
	}
	return nil
}

// UpdatePermission renames a node and/or changes its code. Nil means "leave unchanged".
// Only the domain that owns the node can change it. It returns the node's code after the update.
func UpdatePermission(callerDomainID uint64, code string, newName, newCode *string) (string, error) {
	if db.DB == nil {
		return "", errors.New("database not connected")
	}

	finalCode := code
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPermissionWrites(tx); err != nil {
			return err
		}

		var node models.Permission
		if err := tx.Where("code = ?", code).First(&node).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPermissionNotFound
			}
			return err
		}
		if node.DomainID != callerDomainID {
			return ErrPermissionNotOwned
		}

		updates := map[string]interface{}{}

		if newCode != nil && *newCode != node.Code {
			var count int64
			if err := tx.Model(&models.Permission{}).Where("code = ?", *newCode).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrPermissionCodeTaken
			}
			updates["code"] = *newCode
			finalCode = *newCode
		}

		if newName != nil && *newName != node.Name {
			q := tx.Model(&models.Permission{}).Where("domain_id = ? AND name = ? AND id <> ?", node.DomainID, *newName, node.ID)
			if node.ParentID == nil {
				q = q.Where("parent_id IS NULL")
			} else {
				q = q.Where("parent_id = ?", *node.ParentID)
			}
			var count int64
			if err := q.Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrPermissionNameTaken
			}
			updates["name"] = *newName
		}

		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&node).Updates(updates).Error
	})
	if err != nil {
		return "", err
	}
	return finalCode, nil
}

// GetUserPermissionCodes returns the codes of every feature group, feature and permission the user holds
// through any of their roles. Each code appears once even if several roles grant it. A user whose roles
// have no permissions gets three empty lists.
func GetUserPermissionCodes(userID uint64) (models.PermissionCodeLists, error) {
	if db.DB == nil {
		return models.PermissionCodeLists{}, errors.New("database not connected")
	}

	var ids []uint64
	err := db.DB.Table("role_permissions AS rp").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Pluck("rp.permission_id", &ids).Error
	if err != nil {
		return models.PermissionCodeLists{}, err
	}
	allowed := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}

	var rows []models.Permission
	if err := db.DB.Order("id ASC").Find(&rows).Error; err != nil {
		return models.PermissionCodeLists{}, err
	}
	return models.PermissionCodes(models.FilterPermissionRows(rows, allowed)), nil
}

// GetDomainPermissionCodes returns the codes of everything a domain can see (for shutterx: every permission).
func GetDomainPermissionCodes(dom *models.Domain) (models.PermissionCodeLists, error) {
	if db.DB == nil {
		return models.PermissionCodeLists{}, errors.New("database not connected")
	}

	rows, err := visiblePermissionRows(db.DB, dom)
	if err != nil {
		return models.PermissionCodeLists{}, err
	}
	return models.PermissionCodes(rows), nil
}
