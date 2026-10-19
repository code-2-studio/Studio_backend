package domain

import (
	"errors"
	"studio_backend/db"
	"studio_backend/models"
)

func GetUserByID(id uint64) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var user models.User
	err := db.DB.Preload("Roles").Preload("Roles.Permissions").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByEmail(email string) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var user models.User
	err := db.DB.Preload("Roles").Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetAllUsers(page, pageSize int, keyword string) ([]models.User, int64, error) {
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

	query := db.DB.Model(&models.User{})
	if keyword != "" {
		likeTerm := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ? OR phone LIKE ?", likeTerm, likeTerm, likeTerm)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Roles").
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
	return db.DB.Save(user).Error
}

func DeleteUser(id uint64) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Delete(&models.User{}, id).Error
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
