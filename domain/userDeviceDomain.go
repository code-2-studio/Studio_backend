package domain

import (
	"errors"
	"time"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordUserDevice inserts the device for the user, or refreshes it if it was seen before.
// It reports whether this device was new for the user.
func RecordUserDevice(device *models.UserDevice) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not connected")
	}

	var existing int64
	if err := db.DB.Model(&models.UserDevice{}).
		Where("user_id = ? AND device_id = ?", device.UserID, device.DeviceID).
		Count(&existing).Error; err != nil {
		return false, err
	}

	now := time.Now()
	device.FirstLoginAt = now
	device.LastLoginAt = now
	device.LoginCount = 1

	err := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"device_type":     device.DeviceType,
			"os":              device.OS,
			"os_version":      device.OSVersion,
			"browser":         device.Browser,
			"browser_version": device.BrowserVersion,
			"user_agent":      device.UserAgent,
			"ip_address":      device.IPAddress,
			"last_login_at":   now,
			"logged_out_at":   nil,
			"login_count":     gorm.Expr("user_devices.login_count + 1"),
		}),
	}).Create(device).Error
	if err != nil {
		return false, err
	}
	return existing == 0, nil
}

// MarkDeviceLoggedOut flags the device as logged out. Returns false if the device is unknown.
func MarkDeviceLoggedOut(userID uint64, deviceID string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not connected")
	}

	res := db.DB.Model(&models.UserDevice{}).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Update("logged_out_at", time.Now())
	return res.RowsAffected > 0, res.Error
}

// IsDeviceLoggedOut reports whether the device exists and has been logged out.
func IsDeviceLoggedOut(userID uint64, deviceID string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not connected")
	}

	var count int64
	err := db.DB.Model(&models.UserDevice{}).
		Where("user_id = ? AND device_id = ? AND logged_out_at IS NOT NULL", userID, deviceID).
		Count(&count).Error
	return count > 0, err
}

// GetUserDevices returns all devices a user has logged in from, most recent first.
func GetUserDevices(userID uint64) ([]models.UserDevice, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var devices []models.UserDevice
	err := db.DB.Where("user_id = ?", userID).Order("last_login_at DESC").Find(&devices).Error
	return devices, err
}
