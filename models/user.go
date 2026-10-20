package models

import (
	"regexp"
	"strings"
	"time"
)

// Usernames are stored lowercase and are unique per domain (login is "username@domain").
var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{2,50}$`)
var usernameInvalidChars = regexp.MustCompile(`[^a-z0-9._-]`)

type User struct {
	ID        uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	DomainID  uint64     `json:"-" gorm:"not null;uniqueIndex:idx_users_domain_username,priority:1;uniqueIndex:idx_users_domain_email,priority:1"`
	Username  string     `json:"username" gorm:"size:50;not null;uniqueIndex:idx_users_domain_username,priority:2"`
	Name      string     `json:"name" gorm:"size:100;not null"`
	Email     string     `json:"email" gorm:"size:150;not null;uniqueIndex:idx_users_domain_email,priority:2"`
	Phone     *string    `json:"phone" gorm:"size:20"`
	Password  string     `json:"-" gorm:"size:255;not null"`
	Status    string     `json:"status" gorm:"size:20;default:'active';not null"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// Associations
	Domain *Domain `json:"-" gorm:"foreignKey:DomainID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Roles  []Role  `json:"roles,omitempty" gorm:"many2many:user_roles;foreignKey:ID;joinForeignKey:UserID;References:ID;joinReferences:RoleID"`
}

type UserResponse struct {
	ID        uint64     `json:"id"`
	Domain    string     `json:"domain"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     *string    `json:"phone"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Roles     []Role     `json:"roles,omitempty"`
}

func (u *User) ToResponse() UserResponse {
	domainName := ""
	if u.Domain != nil {
		domainName = u.Domain.Name
	}

	// Roles live in the user's domain; label them with its name
	var roles []Role
	if len(u.Roles) > 0 {
		roles = make([]Role, len(u.Roles))
		copy(roles, u.Roles)
		for i := range roles {
			roles[i].DomainName = domainName
		}
	}

	return UserResponse{
		ID:        u.ID,
		Domain:    domainName,
		Username:  u.Username,
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Status:    u.Status,
		LastLogin: u.LastLogin,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		Roles:     roles,
	}
}

// HasRole reports whether the user holds a role with the given name (Roles must be loaded).
func (u *User) HasRole(name string) bool {
	for _, r := range u.Roles {
		if r.Name == name {
			return true
		}
	}
	return false
}

// IsRootSuperAdmin reports whether the user is a superadmin of the root (shutterx) domain,
// i.e. may act across every child domain. Domain and Roles must be loaded.
func (u *User) IsRootSuperAdmin() bool {
	return u.Domain != nil && u.Domain.IsRoot() && u.HasRole(SuperAdminRoleName)
}

// NormalizeUsername lowercases and trims a client-supplied username.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// ValidUsername reports whether an already-normalized username is acceptable.
func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

// DeriveUsername builds a username from the local part of an email address.
func DeriveUsername(email string) string {
	local := email
	if at := strings.Index(email, "@"); at >= 0 {
		local = email[:at]
	}
	local = usernameInvalidChars.ReplaceAllString(NormalizeUsername(local), "_")
	if len(local) > 50 {
		local = local[:50]
	}
	return local
}

type UserRole struct {
	UserID uint64 `json:"user_id" gorm:"primaryKey"`
	RoleID uint64 `json:"role_id" gorm:"primaryKey"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
