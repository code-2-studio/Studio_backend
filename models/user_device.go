package models

import "time"

// UserDevice records a device a user has logged in from. One row per (user, device_id).
// Currently informational only; a per-user device limit can be enforced on top of it later.
type UserDevice struct {
	ID             uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         uint64     `json:"user_id" gorm:"not null;uniqueIndex:idx_user_device"`
	DeviceID       string     `json:"device_id" gorm:"size:100;not null;uniqueIndex:idx_user_device"`
	DeviceType     string     `json:"device_type" gorm:"size:30"`
	OS             string     `json:"os" gorm:"size:50"`
	OSVersion      string     `json:"os_version" gorm:"size:50"`
	Browser        string     `json:"browser" gorm:"size:50"`
	BrowserVersion string     `json:"browser_version" gorm:"size:50"`
	UserAgent      string     `json:"user_agent" gorm:"size:512"`
	IPAddress      string     `json:"ip_address" gorm:"size:45"`
	LoginCount     uint       `json:"login_count" gorm:"default:1"`
	FirstLoginAt   time.Time  `json:"first_login_at"`
	LastLoginAt    time.Time  `json:"last_login_at"`
	LoggedOutAt    *time.Time `json:"logged_out_at"` // set on logout, cleared on next login
}

func (UserDevice) TableName() string {
	return "user_devices"
}
