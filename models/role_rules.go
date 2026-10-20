package models

import "strings"

const (
	roleNameMaxLen        = 100
	roleDescriptionMaxLen = 255
)

// ValidRoleName reports whether an already-trimmed role name is usable.
func ValidRoleName(name string) bool {
	return name != "" && len(name) <= roleNameMaxLen
}

// ValidRoleDescription reports whether an already-trimmed description fits.
func ValidRoleDescription(description string) bool {
	return len(description) <= roleDescriptionMaxLen
}

// IsReservedRoleName reports whether name is off limits in the given domain. In the root domain the
// "superadmin" role is what grants cross-domain powers, so nobody can create or rename a role to it.
func IsReservedRoleName(dom *Domain, name string) bool {
	return dom != nil && dom.IsRoot() && strings.EqualFold(strings.TrimSpace(name), SuperAdminRoleName)
}
