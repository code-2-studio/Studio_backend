package models

import (
	"time"
)

const (
	PermissionLevelGroup      = "group"
	PermissionLevelFeature    = "feature"
	PermissionLevelPermission = "permission"
)

// Permission is one node of the global permission catalog: a feature group, a feature inside a
// group, or a permission inside a feature. The catalog is stored under the root (shutterx) domain.
// Roles are granted permission-level nodes only. Only name and code are ever returned to clients.
type Permission struct {
	ID        uint64    `json:"-" gorm:"primaryKey;autoIncrement"`
	DomainID  uint64    `json:"-" gorm:"not null;index"`
	Level     string    `json:"-" gorm:"size:20;not null"`
	ParentID  *uint64   `json:"-" gorm:"index"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	Code      string    `json:"code" gorm:"size:32;not null;uniqueIndex"`
	CreatedAt time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"-" gorm:"autoUpdateTime"`

	Domain *Domain     `json:"-" gorm:"foreignKey:DomainID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Parent *Permission `json:"-" gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
