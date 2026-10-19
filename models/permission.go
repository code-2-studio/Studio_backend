package models

import (
	"time"
)

type Permission struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	ModuleName     string    `json:"module_name" gorm:"size:100;not null"`
	PermissionName string    `json:"permission_name" gorm:"size:100;not null"`
	PermissionKey  string    `json:"permission_key" gorm:"size:150;not null;uniqueIndex"`
	Description    *string   `json:"description" gorm:"size:255"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}
