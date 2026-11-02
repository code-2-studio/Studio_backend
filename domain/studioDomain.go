package domain

import (
	"errors"
	"strings"
	"time"

	"studio_backend/db"
	"studio_backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ReservedSubdomains / Slugs that cannot be registered
var ReservedSubdomains = map[string]bool{
	"admin":     true,
	"api":       true,
	"app":       true,
	"auth":      true,
	"billing":   true,
	"crm":       true,
	"dashboard": true,
	"dev":       true,
	"help":      true,
	"live":      true,
	"login":     true,
	"mail":      true,
	"master":    true,
	"root":      true,
	"shutterx":  true,
	"staging":   true,
	"static":    true,
	"status":    true,
	"studio":    true,
	"super":     true,
	"support":   true,
	"system":    true,
	"test":      true,
	"www":       true,
}

// CheckDomainAvailable checks if a studio slug is available
func CheckDomainAvailable(slug string) (bool, string, error) {
	if db.DB == nil {
		return false, "", errors.New("database not connected")
	}

	clean := strings.ToLower(strings.TrimSpace(slug))
	if ReservedSubdomains[clean] {
		return false, "reserved", nil
	}

	var count int64
	err := db.DB.Model(&models.Studio{}).Where("LOWER(name) = ?", clean).Count(&count).Error
	if err != nil {
		return false, "", err
	}
	if count > 0 {
		return false, "taken", nil
	}

	return true, "", nil
}

// CreateStudio creates a new studio, admin role, permissions, and admin user in a single transaction
func CreateStudio(
	studioRecord *models.Studio,
	adminUser *models.User,
	rawPassword string,
) (*models.Studio, *models.User, error) {
	if db.DB == nil {
		return nil, nil, errors.New("database not connected")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, errors.New("failed to hash admin password")
	}
	adminUser.Password = string(hashedPassword)

	var createdStudio models.Studio
	var createdUser models.User

	err = db.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Insert studio (contains all profile fields directly)
		studioRecord.Status = "active"
		if studioRecord.ParentID == nil {
			parentRoot := uint64(1)
			studioRecord.ParentID = &parentRoot
		}
		if err := tx.Create(studioRecord).Error; err != nil {
			return err
		}
		createdStudio = *studioRecord

		// 2. Create Studio "Admin" Role
		roleDescription := "Studio administrator"
		studioAdminRole := models.Role{
			DomainID:    &createdStudio.ID,
			Name:        "Admin",
			Description: &roleDescription,
		}
		if err := tx.Create(&studioAdminRole).Error; err != nil {
			return err
		}

		// 3. Copy default permissions for this studio in batch
		var defaultPermIDs []uint64
		_ = tx.Table("studio_permissions").Where("studio_id = 4").Pluck("permission_id", &defaultPermIDs)
		if len(defaultPermIDs) == 0 {
			_ = tx.Table("permissions").Where("(domain_id = 1 OR domain_id = 0) AND parent_id IS NOT NULL").Pluck("id", &defaultPermIDs)
		}

		now := time.Now()
		var studioPerms []models.StudioPermission
		var rolePerms []models.RolePermission
		for _, permID := range defaultPermIDs {
			studioPerms = append(studioPerms, models.StudioPermission{
				DomainID:     createdStudio.ID,
				PermissionID: permID,
				CreatedAt:    now,
			})
			rolePerms = append(rolePerms, models.RolePermission{
				RoleID:       studioAdminRole.ID,
				PermissionID: permID,
			})
		}
		if len(studioPerms) > 0 {
			if err := tx.Table("studio_permissions").Create(&studioPerms).Error; err != nil {
				return err
			}
		}
		if len(rolePerms) > 0 {
			if err := tx.Table("role_permissions").Create(&rolePerms).Error; err != nil {
				return err
			}
		}

		// 4. Create the studio Admin user
		adminUser.DomainID = &createdStudio.ID
		adminUser.Status = "active"
		if err := tx.Create(adminUser).Error; err != nil {
			return err
		}

		// 5. Assign the Admin role to user
		userRole := models.UserRole{
			UserID: adminUser.ID,
			RoleID: studioAdminRole.ID,
		}
		if err := tx.Table("user_roles").Create(&userRole).Error; err != nil {
			return err
		}

		// Reload user with roles
		if err := tx.Preload("Roles").First(&createdUser, adminUser.ID).Error; err != nil {
			createdUser = *adminUser
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return &createdStudio, &createdUser, nil
}

// GetStudioByID retrieves a studio by its ID
func GetStudioByID(id uint64) (*models.Studio, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var s models.Studio
	if err := db.DB.Where("studio_id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}

	return &s, nil
}

// UpdateStudio updates studio details
func UpdateStudio(id uint64, updates *models.Studio) (*models.Studio, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var s models.Studio
	if err := db.DB.Where("studio_id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}

	updates.ID = id
	if err := db.DB.Model(&s).Where("studio_id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	_ = db.DB.Where("studio_id = ?", id).First(&s)
	return &s, nil
}

// GetAllStudios retrieves paginated studios
func GetAllStudios(page, pageSize int, keyword string) ([]models.Studio, int64, error) {
	if db.DB == nil {
		return nil, 0, errors.New("database not connected")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	var studios []models.Studio
	var total int64

	query := db.DB.Model(&models.Studio{})
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(city) LIKE ?", like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("studio_id ASC").Offset(offset).Limit(pageSize).Find(&studios).Error; err != nil {
		return nil, 0, err
	}

	return studios, total, nil
}

// GetAllDomains is an alias for GetAllStudios
func GetAllDomains(page, pageSize int, keyword string) ([]models.Studio, int64, error) {
	return GetAllStudios(page, pageSize, keyword)
}
