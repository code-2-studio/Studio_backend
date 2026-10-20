package models

import (
	"strings"
	"testing"
)

func TestSetupShortestValidRequestUsesDefaults(t *testing.T) {
	in, issues := ParseSetupSuperAdmin(SetupSuperAdminInput{FullName: "Asha Rao", Password: "longenough"})
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if in.Username != "superadmin" || in.Email != "superadmin@shutterx" || in.Mobile != nil {
		t.Fatalf("defaults wrong: %+v", in)
	}
}

func TestSetupEmailDefaultFollowsTheUsername(t *testing.T) {
	in, issues := ParseSetupSuperAdmin(SetupSuperAdminInput{FullName: "Asha", Username: "Boss", Password: "longenough"})
	if len(issues) > 0 || in.Username != "boss" || in.Email != "boss@shutterx" {
		t.Fatalf("in=%+v issues=%+v", in, issues)
	}
}

func TestSetupCleansExplicitValues(t *testing.T) {
	in, issues := ParseSetupSuperAdmin(SetupSuperAdminInput{
		FullName: "  Asha Rao ", Username: " SuperAdmin ", Email: " Admin@MyCompany.com ",
		Mobile: "+91 98765-43210", Password: "longenough",
	})
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if in.FullName != "Asha Rao" || in.Username != "superadmin" || in.Email != "admin@mycompany.com" ||
		in.Mobile == nil || *in.Mobile != "+919876543210" {
		t.Fatalf("not cleaned: %+v mobile=%v", in, in.Mobile)
	}
}

func TestSetupReportsEveryProblemAtOnce(t *testing.T) {
	_, issues := ParseSetupSuperAdmin(SetupSuperAdminInput{
		FullName: " ", Username: "a b", Email: "not-an-email", Mobile: "12", Password: "short",
	})
	got := map[string]bool{}
	for _, is := range issues {
		got[is.Path] = true
	}
	for _, path := range []string{
		"superadmin.full_name", "superadmin.username", "superadmin.email", "superadmin.mobile", "superadmin.password",
	} {
		if !got[path] {
			t.Errorf("expected an issue at %s, got %+v", path, issues)
		}
	}
}

func TestSetupPasswordLimits(t *testing.T) {
	for _, tc := range []struct {
		pw string
		ok bool
	}{{"", false}, {"1234567", false}, {"12345678", true}, {strings.Repeat("a", 72), true}, {strings.Repeat("a", 73), false}} {
		_, issues := ParseSetupSuperAdmin(SetupSuperAdminInput{FullName: "Asha", Password: tc.pw})
		bad := false
		for _, is := range issues {
			bad = bad || is.Path == "superadmin.password"
		}
		if bad == tc.ok {
			t.Errorf("password of %d chars: ok=%v issues=%+v", len(tc.pw), tc.ok, issues)
		}
	}
}
