package models

import (
	"strings"
	"testing"
)

func TestRoleNameRules(t *testing.T) {
	if ValidRoleName("") || !ValidRoleName("Manager") || ValidRoleName(strings.Repeat("a", 101)) || !ValidRoleName(strings.Repeat("a", 100)) {
		t.Fatalf("role name length rules are wrong")
	}
	if !ValidRoleDescription("") || ValidRoleDescription(strings.Repeat("a", 256)) || !ValidRoleDescription(strings.Repeat("a", 255)) {
		t.Fatalf("role description length rules are wrong")
	}
}

func TestSuperadminRoleNameIsReservedOnlyInTheRootDomain(t *testing.T) {
	rootID := uint64(1)
	root := &Domain{ID: 1, Name: "shutterx"}
	child := &Domain{ID: 2, Name: "acme", ParentID: &rootID}

	for _, name := range []string{"superadmin", "SuperAdmin", "  superadmin "} {
		if !IsReservedRoleName(root, name) {
			t.Errorf("%q must be reserved in the root domain", name)
		}
		if IsReservedRoleName(child, name) {
			t.Errorf("%q is an ordinary role name in a child domain", name)
		}
	}
	if IsReservedRoleName(root, "Admin") || IsReservedRoleName(nil, "superadmin") {
		t.Errorf("only superadmin in the root domain is reserved")
	}
}
