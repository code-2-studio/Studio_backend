package domain

import (
	"errors"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAlreadyInitialized is returned when a shutterx superadmin already exists.
var ErrAlreadyInitialized = errors.New("system is already initialized")

// setupLock serialises first-run setup so two simultaneous calls cannot both create a superadmin.
const setupLock = 7301002

// SetupResult is what first-run setup created.
type SetupResult struct {
	User               models.UserResponse
	PermissionsCreated int
}

// hasRootSuperAdmin reports whether any user holds the superadmin role of the root domain.
func hasRootSuperAdmin(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Table("user_roles AS ur").
		Joins("JOIN roles r ON r.id = ur.role_id").
		Joins("JOIN domains d ON d.id = r.domain_id").
		Where("d.parent_id IS NULL AND r.name = ?", models.SuperAdminRoleName).
		Count(&count).Error
	return count > 0, err
}

// IsInitialized reports whether the system already has a shutterx superadmin.
func IsInitialized() (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not connected")
	}
	return hasRootSuperAdmin(db.DB)
}

// InitializeSystem creates the first superadmin of the root (shutterx) domain: the superadmin role, the
// user, the link between them, and optionally the first permission catalog. Everything is checked before
// anything is written, and it all happens in one transaction, so a rejected request leaves no trace.
// It returns ErrAlreadyInitialized once a superadmin exists.
func InitializeSystem(in models.SetupInput, passwordHash string, groups []models.FeatureGroupItem, isArray bool) (*SetupResult, []models.FieldIssue, error) {
	if db.DB == nil {
		return nil, nil, errors.New("database not connected")
	}

	var result *SetupResult
	var issues []models.FieldIssue

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", setupLock).Error; err != nil {
			return err
		}

		initialized, err := hasRootSuperAdmin(tx)
		if err != nil {
			return err
		}
		if initialized {
			return ErrAlreadyInitialized
		}

		// the root domain is normally created at startup; create it if it is somehow missing
		var root models.Domain
		err = tx.Where("parent_id IS NULL").First(&root).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			displayName := "ShutterX"
			root = models.Domain{Name: models.RootDomainName, DisplayName: &displayName, Status: models.DomainStatusActive}
			err = tx.Create(&root).Error
		}
		if err != nil {
			return err
		}

		// ---- checks (nothing is written until all of them pass) ----
		var plan []models.PlannedPermission
		var existingRows []models.Permission
		if len(groups) > 0 {
			if err := tx.Order("id ASC").Find(&existingRows).Error; err != nil {
				return err
			}
			var planIssues []models.PermissionIssue
			plan, _, planIssues = models.PlanPermissionSave(existingRows, root.ID, groups, isArray)
			for _, is := range planIssues {
				is.Path = "permissions" + prefixPermissionPath(is.Path)
				issues = append(issues, is)
			}
		}

		var taken int64
		if err := tx.Model(&models.User{}).Where("domain_id = ? AND username = ?", root.ID, in.Username).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			issues = append(issues, models.FieldIssue{Path: "superadmin.username", Message: "Username is already used in the shutterx domain", Conflict: true})
		}
		if err := tx.Model(&models.User{}).Where("domain_id = ? AND email = ?", root.ID, in.Email).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			issues = append(issues, models.FieldIssue{Path: "superadmin.email", Message: "Email is already used in the shutterx domain", Conflict: true})
		}
		if len(issues) > 0 {
			return nil
		}

		// ---- writes ----
		description := "Full access to the root domain and every child domain"
		var role models.Role
		err = tx.Where("domain_id = ? AND name = ?", root.ID, models.SuperAdminRoleName).
			Attrs(models.Role{Description: &description}).
			FirstOrCreate(&role, models.Role{DomainID: root.ID, Name: models.SuperAdminRoleName}).Error
		if err != nil {
			return err
		}

		user := models.User{
			DomainID: root.ID,
			Username: in.Username,
			Name:     in.FullName,
			Email:    in.Email,
			Phone:    in.Mobile,
			Password: passwordHash,
			Status:   "active",
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
			return err
		}

		if len(plan) > 0 {
			if err := insertPlannedPermissions(tx, root.ID, existingRows, plan); err != nil {
				return err
			}
		}

		user.Domain = &root
		user.Roles = []models.Role{role}
		result = &SetupResult{User: user.ToResponse(), PermissionsCreated: len(plan)}
		return nil
	})
	return result, issues, err
}

// prefixPermissionPath makes a path from the permission planner relative to the setup body's
// "permissions" field. A single object is addressed as the first item of an array, so
// "featureGroup.code" becomes "[0].featureGroup.code"; array paths already start with "[".
func prefixPermissionPath(path string) string {
	if path == "" {
		return ""
	}
	if path[0] == '[' {
		return path
	}
	return "[0]." + path
}
