package domain

import (
	"errors"
	"studio_backend/db"
	"studio_backend/models"
)

// GetAllRoles lists roles. domainID restricts the list to one domain (0 = all domains).
func GetAllRoles(domainID uint64) ([]models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var roles []models.Role
	err := db.DB.Scopes(inDomain(domainID)).Preload("Domain").Preload("Permissions").Order("id ASC").Find(&roles).Error
	return roles, err
}

// GetRoleByID returns a role. domainID restricts the lookup to one domain (0 = any domain).
func GetRoleByID(id uint64, domainID uint64) (*models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var role models.Role
	err := db.DB.Scopes(inDomain(domainID)).Preload("Domain").Preload("Permissions").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func CreateRole(role *models.Role) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Create(role).Error
}

func AssignPermissionToRole(roleID, permissionID uint64) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	rolePermission := models.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}
	return db.DB.FirstOrCreate(&rolePermission, rolePermission).Error
}

// GetRoleByName finds a role by name inside one domain.
func GetRoleByName(domainID uint64, name string) (*models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var role models.Role
	if err := db.DB.Where("domain_id = ? AND name = ?", domainID, name).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}
