package domain

import (
	"errors"
	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
)

// GetUserByID returns a user. domainID restricts the lookup to one domain (0 = any domain).
func GetUserByID(id uint64, domainID uint64) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var user models.User
	err := db.DB.Scopes(inDomain(domainID)).
		Preload("Domain").Preload("Roles").Preload("Roles.Permissions").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername finds a user by username inside one domain.
func GetUserByUsername(domainID uint64, username string) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var user models.User
	err := db.DB.Preload("Domain").Preload("Roles").
		Where("domain_id = ? AND username = ?", domainID, username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail finds a user by email inside one domain.
func GetUserByEmail(domainID uint64, email string) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var user models.User
	err := db.DB.Preload("Domain").Preload("Roles").
		Where("domain_id = ? AND email = ?", domainID, email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAllUsers lists users. domainID restricts the list to one domain (0 = all domains).
func GetAllUsers(page, pageSize int, keyword string, domainID uint64) ([]models.User, int64, error) {
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

	var users []models.User
	var total int64

	query := db.DB.Model(&models.User{}).Scopes(inDomain(domainID))
	if keyword != "" {
		likeTerm := "%" + keyword + "%"
		query = query.Where("(name LIKE ? OR email LIKE ? OR username LIKE ? OR phone LIKE ?)", likeTerm, likeTerm, likeTerm, likeTerm)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Domain").Preload("Roles").
		Offset(offset).
		Limit(pageSize).
		Order("id DESC").
		Find(&users).Error

	return users, total, err
}

func CreateUser(user *models.User) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Create(user).Error
}

func UpdateUser(user *models.User) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Omit("Domain").Save(user).Error
}

// DeleteUser deletes a user. domainID restricts the delete to one domain (0 = any domain).
func DeleteUser(id uint64, domainID uint64) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}

	res := db.DB.Scopes(inDomain(domainID)).Delete(&models.User{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func AssignRoleToUser(userID, roleID uint64) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	userRole := models.UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	return db.DB.FirstOrCreate(&userRole, userRole).Error
}
