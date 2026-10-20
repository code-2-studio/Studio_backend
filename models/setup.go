package models

import (
	"encoding/json"
	"net/mail"
	"strings"
)

// ---- request (what the first-run setup call sends) -------------------------------------------------

type SetupSuperAdminInput struct {
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
}

// SetupRequest creates the first superadmin of the root (shutterx) domain, and optionally the first
// permission catalog. Permissions is the same JSON the permissions API accepts.
type SetupRequest struct {
	SetupKey    string               `json:"setup_key"`
	SuperAdmin  SetupSuperAdminInput `json:"superadmin"`
	Permissions json.RawMessage      `json:"permissions"`
}

// SetupInput is a validated, cleaned superadmin.
type SetupInput struct {
	FullName string
	Username string
	Email    string
	Mobile   *string
	Password string
}

// ParseSetupSuperAdmin validates the superadmin part of a setup request. Every problem is returned at once.
//
// The username defaults to "superadmin" and the email to "<username>@shutterx", so the shortest valid
// request is just a full name and a password.
func ParseSetupSuperAdmin(in SetupSuperAdminInput) (SetupInput, []FieldIssue) {
	var issues []FieldIssue
	add := func(path, msg string) { issues = append(issues, FieldIssue{Path: path, Message: msg}) }

	out := SetupInput{}

	out.FullName = strings.TrimSpace(in.FullName)
	switch {
	case out.FullName == "":
		add("superadmin.full_name", "Full name is required")
	case len(out.FullName) > 100:
		add("superadmin.full_name", "Full name must be at most 100 characters")
	}

	out.Username = NormalizeUsername(in.Username)
	if out.Username == "" {
		out.Username = SuperAdminRoleName
	}
	if !ValidUsername(out.Username) {
		add("superadmin.username", "Username must be 2-50 characters: lowercase letters, digits, '.', '_' or '-'")
	}

	out.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if out.Email == "" && ValidUsername(out.Username) {
		out.Email = out.Username + "@" + RootDomainName
	}
	if addr, err := mail.ParseAddress(out.Email); err != nil || addr.Address != out.Email || len(out.Email) > 150 {
		add("superadmin.email", "Email is not a valid email address")
	}

	if mobile := strings.TrimSpace(in.Mobile); mobile != "" {
		if cleaned, ok := cleanMobile(mobile); ok {
			out.Mobile = &cleaned
		} else {
			add("superadmin.mobile", "Mobile number must be 10-15 digits, optionally starting with +")
		}
	}

	switch {
	case in.Password == "":
		add("superadmin.password", "Password is required")
	case len(in.Password) < minPasswordLen:
		add("superadmin.password", "Password must be at least 8 characters long")
	case len(in.Password) > maxPasswordLen:
		add("superadmin.password", "Password must be at most 72 characters long")
	default:
		out.Password = in.Password
	}

	return out, issues
}
