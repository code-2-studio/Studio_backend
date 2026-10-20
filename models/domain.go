package models

import (
	"regexp"
	"strings"
	"time"
)

const (
	// RootDomainName is the parent domain. Every other domain is a child of it.
	RootDomainName = "shutterx"
	// SuperAdminRoleName is the role that gets full access to its domain
	// (and, in the root domain, to every child domain as well).
	SuperAdminRoleName = "superadmin"

	DomainStatusActive   = "active"
	DomainStatusInactive = "inactive"
)

// Domain names are stored lowercase: letters, digits and hyphens, 3-50 chars.
// Dots are deliberately not allowed so that "user@host.tld" is never mistaken for "username@domain".
var domainNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// Domain is a tenant. The root domain (shutterx) has no parent; all others have ParentID = root.ID.
// Uniqueness of the name (case-insensitive) and the single-root rule are enforced by DB indexes
// created in db.migrateDomains.
type Domain struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	DisplayName *string   `json:"display_name" gorm:"size:150"`
	ParentID    *uint64   `json:"-" gorm:"index"`
	Status      string    `json:"status" gorm:"size:20;default:'active';not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	Parent *Domain `json:"-" gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// DomainResponse is the API view of a domain: the parent is shown by name, never by id.
type DomainResponse struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	DisplayName *string   `json:"display_name"`
	Parent      *string   `json:"parent"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToResponse converts a domain (with Parent loaded, if it has one) to its API view.
func (d *Domain) ToResponse() DomainResponse {
	var parent *string
	if d.Parent != nil {
		parent = &d.Parent.Name
	}
	return DomainResponse{
		ID:          d.ID,
		Name:        d.Name,
		DisplayName: d.DisplayName,
		Parent:      parent,
		Status:      d.Status,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// IsRoot reports whether this is the parent (shutterx) domain.
func (d *Domain) IsRoot() bool {
	return d.ParentID == nil
}

// NormalizeDomainName lowercases and trims a client-supplied domain name.
func NormalizeDomainName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// ValidDomainName reports whether an already-normalized name is acceptable.
func ValidDomainName(name string) bool {
	return domainNamePattern.MatchString(name)
}
