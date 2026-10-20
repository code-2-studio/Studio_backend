package domain

import (
	"errors"
	"strings"

	"studio_backend/db"
	"studio_backend/models"

	"gorm.io/gorm"
)

// inDomain restricts a query to one domain. domainID 0 means "all domains" and is only ever
// passed for a root superadmin (see controllers.domainScope).
func inDomain(domainID uint64) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		if domainID == 0 {
			return q
		}
		return q.Where("domain_id = ?", domainID)
	}
}

func GetRootDomain() (*models.Domain, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var root models.Domain
	if err := db.DB.Where("parent_id IS NULL").First(&root).Error; err != nil {
		return nil, err
	}
	return &root, nil
}

func GetDomainByID(id uint64) (*models.Domain, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var dom models.Domain
	if err := db.DB.Preload("Parent").First(&dom, id).Error; err != nil {
		return nil, err
	}
	return &dom, nil
}

func GetDomainByName(name string) (*models.Domain, error) {
	if db.DB == nil {
		return nil, errors.New("database not connected")
	}

	var dom models.Domain
	if err := db.DB.Preload("Parent").Where("lower(name) = ?", models.NormalizeDomainName(name)).First(&dom).Error; err != nil {
		return nil, err
	}
	return &dom, nil
}

func GetAllDomains(page, pageSize int, keyword string) ([]models.Domain, int64, error) {
	if db.DB == nil {
		return nil, 0, errors.New("database not connected")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var domains []models.Domain
	var total int64

	query := db.DB.Model(&models.Domain{})
	if keyword != "" {
		likeTerm := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("lower(name) LIKE ? OR lower(display_name) LIKE ?", likeTerm, likeTerm)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Parent").Offset((page - 1) * pageSize).Limit(pageSize).Order("id ASC").Find(&domains).Error
	return domains, total, err
}

func CreateDomain(dom *models.Domain) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Create(dom).Error
}

func UpdateDomain(dom *models.Domain) error {
	if db.DB == nil {
		return errors.New("database not connected")
	}
	return db.DB.Omit("Parent").Save(dom).Error
}
