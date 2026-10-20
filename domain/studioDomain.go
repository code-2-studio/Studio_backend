package domain

import (
	"errors"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
)

const defaultAdminRoleDescription = "Studio administrator"

// DomainNameAvailable reports whether no domain uses the (normalized) name yet.
func DomainNameAvailable(name string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not connected")
	}

	var count int64
	if err := db.DB.Model(&models.Domain{}).Where("lower(name) = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// OnboardStudio creates a new studio in one transaction: the domain (a child of parent), its profile,
// the Admin role, and the admin user holding that role. Either everything is created or nothing is.
// If the domain name is taken, nothing is written and the issue is returned.
func OnboardStudio(parent *models.Domain, in models.StudioInput, passwordHash string) (*models.StudioResponse, []models.FieldIssue, error) {
	if db.DB == nil {
		return nil, nil, errors.New("database not connected")
	}

	taken := []models.FieldIssue{{Path: "studio.domain", Message: "Domain name is already taken", Conflict: true}}

	var response *models.StudioResponse
	var issues []models.FieldIssue

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Domain{}).Where("lower(name) = ?", in.Domain).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			issues = taken
			return nil
		}

		studioName := in.StudioName
		dom := models.Domain{
			Name:        in.Domain,
			DisplayName: &studioName,
			ParentID:    &parent.ID,
			Status:      models.DomainStatusActive,
		}
		if err := tx.Create(&dom).Error; err != nil {
			return err
		}

		profile := models.DomainProfile{
			DomainID:      dom.ID,
			OwnerName:     in.OwnerName,
			OwnerMobile:   in.OwnerMobile,
			BusinessEmail: in.BusinessEmail,
			Address:       in.Address,
			City:          in.City,
			State:         in.State,
			Pincode:       in.Pincode,
			Whatsapp:      in.Whatsapp,
			GSTIN:         in.GSTIN,
			Website:       in.Website,
			Instagram:     in.Instagram,
			Facebook:      in.Facebook,
			YouTube:       in.YouTube,
		}
		if err := tx.Create(&profile).Error; err != nil {
			return err
		}

		description := defaultAdminRoleDescription
		role := models.Role{DomainID: dom.ID, Name: models.AdminRoleName, Description: &description}
		if err := tx.Create(&role).Error; err != nil {
			return err
		}

		mobile := in.OwnerMobile
		admin := models.User{
			DomainID: dom.ID,
			Username: in.AdminUsername,
			Name:     in.OwnerName,
			Email:    in.BusinessEmail,
			Phone:    &mobile,
			Password: passwordHash,
			Status:   "active",
		}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.UserRole{UserID: admin.ID, RoleID: role.ID}).Error; err != nil {
			return err
		}

		dom.Parent = parent
		admin.Domain = &dom
		admin.Roles = []models.Role{role}
		built := models.BuildStudioResponse(&dom, &profile, &admin)
		response = &built
		return nil
	})
	if err != nil {
		// a concurrent request may have taken the name between the check and the insert
		if available, checkErr := DomainNameAvailable(in.Domain); checkErr == nil && !available {
			return nil, taken, nil
		}
		return nil, nil, err
	}
	return response, issues, nil
}

// GetStudio returns a domain with its saved owner and studio details. It returns nil when the domain
// does not exist. Owner and Studio are null in the result when the domain has no saved profile.
func GetStudio(domainID uint64) (*models.StudioProfileResponse, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var dom models.Domain
	if err := db.DB.Preload("Parent").First(&dom, domainID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var profile models.DomainProfile
	err := db.DB.Where("domain_id = ?", domainID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		built := models.BuildStudioProfileResponse(&dom, nil)
		return &built, nil
	}
	if err != nil {
		return nil, err
	}

	built := models.BuildStudioProfileResponse(&dom, &profile)
	return &built, nil
}

// UpdateStudio replaces a studio's owner and studio details (and its display name) in one transaction.
// A domain that has no saved profile yet gets one. It returns nil when the domain does not exist.
// The domain name and the admin user are never touched.
func UpdateStudio(domainID uint64, in models.StudioInput) (*models.StudioProfileResponse, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var result *models.StudioProfileResponse
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var dom models.Domain
		if err := tx.Preload("Parent").First(&dom, domainID).Error; err != nil {
			return err
		}

		studioName := in.StudioName
		dom.DisplayName = &studioName
		if err := tx.Omit("Parent").Save(&dom).Error; err != nil {
			return err
		}

		var profile models.DomainProfile
		err := tx.Where("domain_id = ?", domainID).First(&profile).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		exists := err == nil

		profile.DomainID = domainID
		profile.OwnerName = in.OwnerName
		profile.OwnerMobile = in.OwnerMobile
		profile.BusinessEmail = in.BusinessEmail
		profile.Address = in.Address
		profile.City = in.City
		profile.State = in.State
		profile.Pincode = in.Pincode
		profile.Whatsapp = in.Whatsapp
		profile.GSTIN = in.GSTIN
		profile.Website = in.Website
		profile.Instagram = in.Instagram
		profile.Facebook = in.Facebook
		profile.YouTube = in.YouTube

		if exists {
			err = tx.Omit("Domain").Save(&profile).Error
		} else {
			err = tx.Create(&profile).Error
		}
		if err != nil {
			return err
		}

		built := models.BuildStudioProfileResponse(&dom, &profile)
		result = &built
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return result, err
}
