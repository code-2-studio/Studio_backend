package models

import (
	"time"
)

type User struct {
	ID        uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string     `json:"name" gorm:"size:100;not null"`
	Email     string     `json:"email" gorm:"size:150;not null;uniqueIndex"`
	Phone     *string    `json:"phone" gorm:"size:20"`
	Password  string     `json:"-" gorm:"size:255;not null"`
	Status    string     `json:"status" gorm:"size:20;default:'active';not null"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// Associations
	Roles []Role `json:"roles,omitempty" gorm:"many2many:user_roles;foreignKey:ID;joinForeignKey:UserID;References:ID;joinReferences:RoleID"`
}

type UserResponse struct {
	ID        uint64     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     *string    `json:"phone"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Roles     []Role     `json:"roles,omitempty"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Status:    u.Status,
		LastLogin: u.LastLogin,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		Roles:     u.Roles,
	}
}

type UserRole struct {
	UserID uint64 `json:"user_id" gorm:"primaryKey"`
	RoleID uint64 `json:"role_id" gorm:"primaryKey"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
