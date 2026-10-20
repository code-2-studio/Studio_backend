package models

import (
	"strings"
	"testing"
)

func validStudio() StudioRequest {
	return StudioRequest{
		Owner: StudioOwnerInput{FullName: "Asha Rao", Mobile: "+91 98765-43210", BusinessEmail: "Asha@MyStudio.com"},
		Studio: StudioDetailsInput{
			Name: "My Studio", Domain: "MyStudio", Address: "12 MG Road", City: "Pune", State: "Maharashtra", Pincode: "411001",
			Whatsapp: "9876543210", GSTIN: "27abcde1234f1z5", Website: "https://", Instagram: "mystudio",
			Facebook: "https://facebook.com/mystudio", YouTube: "@mystudio",
		},
		Admin: StudioAdminInput{Password: "longenough"},
	}
}

func TestStudioValidRequestIsCleaned(t *testing.T) {
	in, issues := ParseStudioRequest(validStudio())
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if in.Domain != "mystudio" || in.BusinessEmail != "asha@mystudio.com" || in.OwnerMobile != "+919876543210" {
		t.Fatalf("not normalised: %+v", in)
	}
	if in.AdminUsername != "admin" {
		t.Fatalf("admin username should default to admin, got %q", in.AdminUsername)
	}
	if in.Website != nil {
		t.Fatalf("placeholder https:// must count as blank, got %q", *in.Website)
	}
	if in.GSTIN == nil || *in.GSTIN != "27ABCDE1234F1Z5" {
		t.Fatalf("gstin should be upper-cased, got %v", in.GSTIN)
	}
	if in.Instagram == nil || *in.Instagram != "@mystudio" {
		t.Fatalf("instagram handle should get an @, got %v", in.Instagram)
	}
}

func TestStudioOptionalFieldsMayBeBlank(t *testing.T) {
	req := validStudio()
	req.Studio.Whatsapp, req.Studio.GSTIN, req.Studio.Website = "", "", ""
	req.Studio.Instagram, req.Studio.Facebook, req.Studio.YouTube = "", "", ""
	in, issues := ParseStudioRequest(req)
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if in.Whatsapp != nil || in.GSTIN != nil || in.Website != nil || in.Instagram != nil || in.Facebook != nil || in.YouTube != nil {
		t.Fatalf("blank optional fields must stay nil: %+v", in)
	}
}

func TestStudioWebsiteWithoutSchemeGetsHTTPS(t *testing.T) {
	req := validStudio()
	req.Studio.Website = "mystudio.com"
	in, issues := ParseStudioRequest(req)
	if len(issues) > 0 || in.Website == nil || *in.Website != "https://mystudio.com" {
		t.Fatalf("website=%v issues=%+v", in.Website, issues)
	}
}

func TestStudioRejectsBadValuesAndReportsAllAtOnce(t *testing.T) {
	req := StudioRequest{
		Owner:  StudioOwnerInput{FullName: " ", Mobile: "12345", BusinessEmail: "not-an-email"},
		Studio: StudioDetailsInput{Name: "", Domain: "My Studio!", Pincode: "12", GSTIN: "BADGST", Website: "ht!tp://x", Instagram: "bad handle"},
		Admin:  StudioAdminInput{Username: "A B", Password: "short"},
	}
	_, issues := ParseStudioRequest(req)

	got := map[string]bool{}
	for _, is := range issues {
		got[is.Path] = true
	}
	for _, path := range []string{
		"owner.full_name", "owner.mobile", "owner.business_email",
		"studio.name", "studio.domain", "studio.address", "studio.city", "studio.state", "studio.pincode",
		"studio.gstin", "studio.website", "studio.instagram", "admin.username", "admin.password",
	} {
		if !got[path] {
			t.Errorf("expected an issue at %s, got %+v", path, issues)
		}
	}
}

func TestStudioDomainRules(t *testing.T) {
	cases := map[string]bool{
		"mystudio": true, "my-studio": true, "studio2": true,
		"ab": false, "my_studio": false, "my.studio": false, "-bad": false, "admin": false, "www": false,
	}
	for domain, wantOK := range cases {
		req := validStudio()
		req.Studio.Domain = domain
		_, issues := ParseStudioRequest(req)
		hasDomainIssue := false
		for _, is := range issues {
			hasDomainIssue = hasDomainIssue || is.Path == "studio.domain"
		}
		if hasDomainIssue == wantOK {
			t.Errorf("domain %q: ok=%v but issues=%+v", domain, wantOK, issues)
		}
	}
}

func TestStudioPasswordLimits(t *testing.T) {
	for _, tc := range []struct {
		pw string
		ok bool
	}{{"1234567", false}, {"12345678", true}, {strings.Repeat("a", 72), true}, {strings.Repeat("a", 73), false}} {
		req := validStudio()
		req.Admin.Password = tc.pw
		_, issues := ParseStudioRequest(req)
		bad := false
		for _, is := range issues {
			bad = bad || is.Path == "admin.password"
		}
		if bad == tc.ok {
			t.Errorf("password of %d chars: ok=%v issues=%+v", len(tc.pw), tc.ok, issues)
		}
	}
}

func TestStudioResponseShape(t *testing.T) {
	name := "My Studio"
	parentID := uint64(1)
	dom := &Domain{ID: 5, Name: "mystudio", DisplayName: &name, ParentID: &parentID, Status: DomainStatusActive,
		Parent: &Domain{ID: 1, Name: "shutterx"}}
	profile := &DomainProfile{OwnerName: "Asha Rao", OwnerMobile: "+919876543210", BusinessEmail: "asha@mystudio.com",
		Address: "12 MG Road", City: "Pune", State: "Maharashtra", Pincode: "411001"}
	admin := &User{ID: 9, DomainID: 5, Username: "admin", Name: "Asha Rao", Email: "asha@mystudio.com",
		Domain: dom, Roles: []Role{{ID: 2, Name: AdminRoleName}}}

	resp := BuildStudioResponse(dom, profile, admin)
	if resp.Admin.Login != "admin@mystudio" || resp.Admin.Role != "Admin" || resp.Domain.Parent == nil || *resp.Domain.Parent != "shutterx" {
		t.Fatalf("bad response: %+v", resp)
	}
	if resp.Studio.Name != "My Studio" || resp.Studio.Domain != "mystudio" {
		t.Fatalf("studio block wrong: %+v", resp.Studio)
	}
}

func validProfileEdit() StudioProfileRequest {
	req := validStudio()
	return StudioProfileRequest{Owner: req.Owner, Studio: req.Studio}
}

func TestStudioEditUsesTheSameRulesWithoutAdmin(t *testing.T) {
	in, issues := ParseStudioProfileRequest(validProfileEdit(), "mystudio")
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if in.Domain != "mystudio" || in.StudioName != "My Studio" || in.AdminPassword != "" {
		t.Fatalf("edit input wrong: %+v", in)
	}

	bad := validProfileEdit()
	bad.Owner.BusinessEmail, bad.Studio.Pincode, bad.Studio.City = "nope", "12", ""
	_, issues = ParseStudioProfileRequest(bad, "mystudio")
	got := map[string]bool{}
	for _, is := range issues {
		got[is.Path] = true
	}
	for _, path := range []string{"owner.business_email", "studio.pincode", "studio.city"} {
		if !got[path] {
			t.Errorf("expected an issue at %s, got %+v", path, issues)
		}
	}
}

func TestStudioEditLocksTheDomain(t *testing.T) {
	// left out, or repeated in any case: fine
	for _, sent := range []string{"", "mystudio", "MyStudio"} {
		req := validProfileEdit()
		req.Studio.Domain = sent
		if _, issues := ParseStudioProfileRequest(req, "mystudio"); len(issues) > 0 {
			t.Errorf("domain %q should be accepted, got %+v", sent, issues)
		}
	}
	// a different one: rejected
	req := validProfileEdit()
	req.Studio.Domain = "other"
	_, issues := ParseStudioProfileRequest(req, "mystudio")
	if len(issues) != 1 || issues[0].Path != "studio.domain" || issues[0].Message != "The domain cannot be changed" {
		t.Fatalf("changing the domain must be rejected, got %+v", issues)
	}
}

func TestStudioEditCanClearOptionalFields(t *testing.T) {
	req := validProfileEdit()
	req.Studio.Whatsapp, req.Studio.GSTIN, req.Studio.Website = "", "", ""
	req.Studio.Instagram, req.Studio.Facebook, req.Studio.YouTube = "", "", ""
	in, issues := ParseStudioProfileRequest(req, "mystudio")
	if len(issues) > 0 || in.Whatsapp != nil || in.GSTIN != nil || in.Website != nil ||
		in.Instagram != nil || in.Facebook != nil || in.YouTube != nil {
		t.Fatalf("blank optional fields must come back nil: %+v issues=%+v", in, issues)
	}
}

func TestStudioProfileResponseHandlesMissingProfile(t *testing.T) {
	name := "ShutterX"
	root := &Domain{ID: 1, Name: "shutterx", DisplayName: &name}
	resp := BuildStudioProfileResponse(root, nil)
	if resp.Owner != nil || resp.Studio != nil || resp.Domain.Name != "shutterx" {
		t.Fatalf("a domain without a profile should have null owner and studio: %+v", resp)
	}

	studioName := "My Studio"
	dom := &Domain{ID: 5, Name: "mystudio", DisplayName: &studioName}
	profile := &DomainProfile{OwnerName: "Asha Rao", OwnerMobile: "+919876543210", BusinessEmail: "asha@mystudio.com",
		Address: "12 MG Road", City: "Pune", State: "Maharashtra", Pincode: "411001"}
	resp = BuildStudioProfileResponse(dom, profile)
	if resp.Owner == nil || resp.Studio == nil || resp.Owner.FullName != "Asha Rao" ||
		resp.Studio.Name != "My Studio" || resp.Studio.Domain != "mystudio" || resp.Studio.Whatsapp != nil {
		t.Fatalf("profile response wrong: %+v", resp)
	}
}
