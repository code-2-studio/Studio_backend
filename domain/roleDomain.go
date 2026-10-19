package domain

import (
	"errors"
	"studio_backend/db"
	"studio_backend/models"
)

func GetAllRoles() ([]models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var roles []models.Role
	err := db.DB.Preload("Permissions").Order("id ASC").Find(&roles).Error
	return roles, err
}

func GetRoleByID(id uint64) (*models.Role, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var role models.Role
	err := db.DB.Preload("Permissions").First(&role, id).Error
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

func GetAllPermissions() ([]models.Permission, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var permissions []models.Permission
	err := db.DB.Order("module_name ASC, id ASC").Find(&permissions).Error
	return permissions, err
}
