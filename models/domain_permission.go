package models

import "time"

// Share modes for PUT /permissions/share.
const (
	ShareModeReplace = "replace" // the request becomes the domain's complete set
	ShareModeAdd     = "add"     // add the requested permissions, keep the rest
	ShareModeRemove  = "remove"  // remove the requested permissions, keep the rest
	ShareModeClear   = "clear"   // remove everything shared with the domain
)

// DomainPermission records that a permission of the shutterx catalog is shared with a child domain.
// Only permission-level nodes are stored; their groups and features are shown as parents.
// The root domain has no rows because it sees the whole catalog.
type DomainPermission struct {
	DomainID     uint64    `json:"-" gorm:"primaryKey"`
	PermissionID uint64    `json:"-" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"-" gorm:"autoCreateTime"`

	Domain     *Domain     `json:"-" gorm:"foreignKey:DomainID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Permission *Permission `json:"-" gorm:"foreignKey:PermissionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (DomainPermission) TableName() string {
	return "domain_permissions"
}
