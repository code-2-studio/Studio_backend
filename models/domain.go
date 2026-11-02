package models

import "time"

type Domain struct {
	ID            uint64    `json:"id" gorm:"primaryKey;column:studio_id;autoIncrement"`
	Name          string    `json:"slug" gorm:"column:name;size:100;not null;uniqueIndex"`
	DisplayName   *string   `json:"name" gorm:"column:display_name;size:150"`
	ParentID      *uint64   `json:"parent_id,omitempty" gorm:"column:parent_id"`
	Status        string    `json:"status" gorm:"column:status;size:20;default:'active';not null"`
	OwnerName     string    `json:"owner_name" gorm:"column:owner_name;size:100"`
	OwnerMobile   string    `json:"owner_mobile" gorm:"column:owner_mobile;size:20"`
	BusinessEmail string    `json:"business_email" gorm:"column:business_email;size:150"`
	Address       string    `json:"address" gorm:"column:address;size:255"`
	City          string    `json:"city" gorm:"column:city;size:100"`
	State         string    `json:"state" gorm:"column:state;size:100"`
	Pincode       string    `json:"pincode" gorm:"column:pincode;size:20"`
	WhatsApp      *string   `json:"whatsapp" gorm:"column:whatsapp;size:20"`
	GSTIN         *string   `json:"gstin" gorm:"column:gstin;size:20"`
	Website       *string   `json:"website" gorm:"column:website;size:255"`
	Instagram     *string   `json:"instagram" gorm:"column:instagram;size:255"`
	Facebook      *string   `json:"facebook" gorm:"column:facebook;size:255"`
	YouTube       *string   `json:"youtube" gorm:"column:youtube;size:255"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Domain) TableName() string {
	return "studios"
}

type DomainPermission struct {
	DomainID     uint64    `json:"domain_id" gorm:"primaryKey;column:studio_id"`
	PermissionID uint64    `json:"permission_id" gorm:"primaryKey;column:permission_id"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (DomainPermission) TableName() string {
	return "studio_permissions"
}

// Aliases for studio terminology
type Studio = Domain
type StudioPermission = DomainPermission
