package models

import (
	"time"
)

type Role struct {
	ID          uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string       `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Description *string      `json:"description" gorm:"size:255"`
	CreatedAt   time.Time    `json:"created_at" gorm:"autoCreateTime"`
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;foreignKey:ID;joinForeignKey:RoleID;References:ID;joinReferences:PermissionID"`
}

type RolePermission struct {
	RoleID       uint64 `json:"role_id" gorm:"primaryKey"`
	PermissionID uint64 `json:"permission_id" gorm:"primaryKey"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
