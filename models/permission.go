package models

import (
	"time"
)

type Permission struct {
	ID          uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	DomainID    uint64     `json:"domain_id" gorm:"not null"`
	ParentID    *uint64    `json:"parent_id"`
	Code        string     `json:"code" gorm:"size:50;not null"`
	Name        string     `json:"name" gorm:"size:100;not null"`
	Level       string     `json:"level" gorm:"size:50;not null"`
	Description *string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
