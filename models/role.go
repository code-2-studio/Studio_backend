package models

import (
	"time"

	"gorm.io/gorm"
)

type Role struct {
	ID          uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	DomainID    uint64       `json:"-" gorm:"not null;uniqueIndex:idx_roles_domain_name,priority:1"`
	DomainName  string       `json:"domain,omitempty" gorm:"-"`
	Name        string       `json:"name" gorm:"size:100;not null;uniqueIndex:idx_roles_domain_name,priority:2"`
	Description *string      `json:"description" gorm:"size:255"`
	CreatedAt   time.Time    `json:"created_at" gorm:"autoCreateTime"`
	Domain      *Domain      `json:"-" gorm:"foreignKey:DomainID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;foreignKey:ID;joinForeignKey:RoleID;References:ID;joinReferences:PermissionID"`
}

// AfterFind fills DomainName when the role was loaded with its Domain.
func (r *Role) AfterFind(_ *gorm.DB) error {
	if r.Domain != nil {
		r.DomainName = r.Domain.Name
	}
	return nil
}

type RolePermission struct {
	RoleID       uint64 `json:"role_id" gorm:"primaryKey"`
	PermissionID uint64 `json:"permission_id" gorm:"primaryKey"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
