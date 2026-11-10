package models

import "time"

type Location struct {
	LocationID uint64    `json:"location_id" gorm:"primaryKey;column:location_id;autoIncrement"`
	StudioID   uint64    `json:"studio_id" gorm:"column:studio_id;not null;index"`
	Name       string    `json:"name" gorm:"column:name;size:150;not null"`
	IsPrimary  bool      `json:"is_primary" gorm:"column:is_primary;default:true;not null"`
	Address    string    `json:"address" gorm:"column:address;size:255;not null"`
	City       string    `json:"city" gorm:"column:city;size:100;not null"`
	State      string    `json:"state" gorm:"column:state;size:100;not null"`
	Pincode    string    `json:"pincode" gorm:"column:pincode;size:20;not null"`
	Phone      *string   `json:"phone" gorm:"column:phone;size:20"`
	Status     string    `json:"status" gorm:"column:status;size:20;default:'active';not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Location) TableName() string {
	return "locations"
}

