package models

import (
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AdminRoleName is the role created for the first user of every onboarded studio.
const AdminRoleName = "Admin"

// FieldIssue is one problem with a request field, located by its JSON path. It shares its shape with
// PermissionIssue so every endpoint reports problems the same way.
type FieldIssue = PermissionIssue

// reservedDomainNames cannot be used as a studio domain.
var reservedDomainNames = map[string]bool{
	"admin": true, "api": true, "app": true, "www": true, "mail": true, "root": true,
	"support": true, "help": true, "static": true, "assets": true, "login": true,
	"register": true, "dashboard": true, "superadmin": true, "studio": true,
}

// IsReservedDomainName reports whether a (normalized) domain name is off limits.
func IsReservedDomainName(name string) bool {
	return reservedDomainNames[name]
}

var (
	mobilePattern  = regexp.MustCompile(`^\+?[0-9]{10,15}$`)
	pincodePattern = regexp.MustCompile(`^[0-9]{6}$`)
	gstinPattern   = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
	instagramRe    = regexp.MustCompile(`^@?[A-Za-z0-9._]{1,30}$`)
	otherSocialRe  = regexp.MustCompile(`^@?[A-Za-z0-9._-]{1,100}$`)
	mobileCleaner  = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "")
)

const (
	maxPasswordLen = 72 // bcrypt ignores or rejects anything longer
	minPasswordLen = 8
)

// ---- request (what the frontend sends) --------------------------------------------------------

type StudioOwnerInput struct {
	FullName      string `json:"full_name"`
	Mobile        string `json:"mobile"`
	BusinessEmail string `json:"business_email"`
}

type StudioDetailsInput struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Address   string `json:"address"`
	City      string `json:"city"`
	State     string `json:"state"`
	Pincode   string `json:"pincode"`
	Whatsapp  string `json:"whatsapp"`
	GSTIN     string `json:"gstin"`
	Website   string `json:"website"`
	Instagram string `json:"instagram"`
	Facebook  string `json:"facebook"`
	YouTube   string `json:"youtube"`
}

type StudioAdminInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type StudioRequest struct {
	Owner  StudioOwnerInput   `json:"owner"`
	Studio StudioDetailsInput `json:"studio"`
	Admin  StudioAdminInput   `json:"admin"`
}

// StudioInput is a validated, cleaned StudioRequest. Optional values are nil when left blank.
type StudioInput struct {
	OwnerName     string
	OwnerMobile   string
	BusinessEmail string
	StudioName    string
	Domain        string
	Address       string
	City          string
	State         string
	Pincode       string
	Whatsapp      *string
	GSTIN         *string
	Website       *string
	Instagram     *string
	Facebook      *string
	YouTube       *string
	AdminUsername string
	AdminPassword string
}

// ---- storage ------------------------------------------------------------------------------------

// DomainProfile holds the business details collected when a studio is onboarded. One row per domain.
// The studio name itself is the domain's display name.
type DomainProfile struct {
	DomainID      uint64    `json:"-" gorm:"primaryKey"`
	OwnerName     string    `json:"-" gorm:"size:100;not null"`
	OwnerMobile   string    `json:"-" gorm:"size:20;not null"`
	BusinessEmail string    `json:"-" gorm:"size:150;not null"`
	Address       string    `json:"-" gorm:"size:255;not null"`
	City          string    `json:"-" gorm:"size:100;not null"`
	State         string    `json:"-" gorm:"size:100;not null"`
	Pincode       string    `json:"-" gorm:"size:10;not null"`
	Whatsapp      *string   `json:"-" gorm:"size:20"`
	GSTIN         *string   `json:"-" gorm:"size:15"`
	Website       *string   `json:"-" gorm:"size:255"`
	Instagram     *string   `json:"-" gorm:"size:255"`
	Facebook      *string   `json:"-" gorm:"size:255"`
	YouTube       *string   `json:"-" gorm:"size:255"`
	CreatedAt     time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"-" gorm:"autoUpdateTime"`

	Domain *Domain `json:"-" gorm:"foreignKey:DomainID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (DomainProfile) TableName() string {
	return "domain_profiles"
}

// ---- response -----------------------------------------------------------------------------------

type StudioOwnerResponse struct {
	FullName      string `json:"full_name"`
	Mobile        string `json:"mobile"`
	BusinessEmail string `json:"business_email"`
}

type StudioDetailsResponse struct {
	Name      string  `json:"name"`
	Domain    string  `json:"domain"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	Pincode   string  `json:"pincode"`
	Whatsapp  *string `json:"whatsapp"`
	GSTIN     *string `json:"gstin"`
	Website   *string `json:"website"`
	Instagram *string `json:"instagram"`
	Facebook  *string `json:"facebook"`
	YouTube   *string `json:"youtube"`
}

type StudioAdminResponse struct {
	Login    string       `json:"login"` // username@domain, what the admin types to sign in
	Username string       `json:"username"`
	Role     string       `json:"role"`
	User     UserResponse `json:"user"`
}

type StudioResponse struct {
	Domain DomainResponse        `json:"domain"`
	Owner  StudioOwnerResponse   `json:"owner"`
	Studio StudioDetailsResponse `json:"studio"`
	Admin  StudioAdminResponse   `json:"admin"`
}

func ownerResponse(p *DomainProfile) StudioOwnerResponse {
	return StudioOwnerResponse{FullName: p.OwnerName, Mobile: p.OwnerMobile, BusinessEmail: p.BusinessEmail}
}

func detailsResponse(dom *Domain, p *DomainProfile) StudioDetailsResponse {
	studioName := ""
	if dom.DisplayName != nil {
		studioName = *dom.DisplayName
	}
	return StudioDetailsResponse{
		Name:      studioName,
		Domain:    dom.Name,
		Address:   p.Address,
		City:      p.City,
		State:     p.State,
		Pincode:   p.Pincode,
		Whatsapp:  p.Whatsapp,
		GSTIN:     p.GSTIN,
		Website:   p.Website,
		Instagram: p.Instagram,
		Facebook:  p.Facebook,
		YouTube:   p.YouTube,
	}
}

// BuildStudioResponse assembles the onboarding result. dom.Parent and admin.Roles should be loaded.
func BuildStudioResponse(dom *Domain, p *DomainProfile, admin *User) StudioResponse {
	role := ""
	if len(admin.Roles) > 0 {
		role = admin.Roles[0].Name
	}
	return StudioResponse{
		Domain: dom.ToResponse(),
		Owner:  ownerResponse(p),
		Studio: detailsResponse(dom, p),
		Admin: StudioAdminResponse{
			Login:    admin.Username + "@" + dom.Name,
			Username: admin.Username,
			Role:     role,
			User:     admin.ToResponse(),
		},
	}
}

// StudioProfileResponse is a studio as read back later: the domain plus its saved owner and studio
// details. Owner and Studio are null for a domain that has no saved profile (for example shutterx itself).
type StudioProfileResponse struct {
	Domain DomainResponse         `json:"domain"`
	Owner  *StudioOwnerResponse   `json:"owner"`
	Studio *StudioDetailsResponse `json:"studio"`
}

// BuildStudioProfileResponse builds the read view; p may be nil. dom.Parent should be loaded.
func BuildStudioProfileResponse(dom *Domain, p *DomainProfile) StudioProfileResponse {
	resp := StudioProfileResponse{Domain: dom.ToResponse()}
	if p != nil {
		owner, details := ownerResponse(p), detailsResponse(dom, p)
		resp.Owner, resp.Studio = &owner, &details
	}
	return resp
}

// ---- validation ---------------------------------------------------------------------------------

// StudioProfileRequest is what the frontend sends to edit a studio after onboarding: the same owner and
// studio objects as onboarding, without the admin section. The domain is locked.
type StudioProfileRequest struct {
	Owner  StudioOwnerInput   `json:"owner"`
	Studio StudioDetailsInput `json:"studio"`
}

// ParseStudioProfileRequest validates an edit with the same rules as onboarding. studio.domain may be
// left out or repeated unchanged; sending a different one is an error. Every problem is returned at once.
func ParseStudioProfileRequest(req StudioProfileRequest, currentDomain string) (StudioInput, []FieldIssue) {
	in, issues := parseOwnerAndStudio(StudioRequest{Owner: req.Owner, Studio: req.Studio}, false)
	if sent := NormalizeDomainName(req.Studio.Domain); sent != "" && sent != currentDomain {
		issues = append(issues, FieldIssue{Path: "studio.domain", Message: "The domain cannot be changed"})
	}
	in.Domain = currentDomain
	return in, issues
}

// ParseStudioRequest validates and cleans an onboarding request. Every problem is returned at once.
func ParseStudioRequest(req StudioRequest) (StudioInput, []FieldIssue) {
	in, issues := parseOwnerAndStudio(req, true)
	add := func(path, msg string) { issues = append(issues, FieldIssue{Path: path, Message: msg}) }

	// admin
	in.AdminUsername = NormalizeUsername(req.Admin.Username)
	if in.AdminUsername == "" {
		in.AdminUsername = "admin"
	}
	if !ValidUsername(in.AdminUsername) {
		add("admin.username", "Username must be 2-50 characters: lowercase letters, digits, '.', '_' or '-'")
	}

	switch {
	case req.Admin.Password == "":
		add("admin.password", "Password is required")
	case len(req.Admin.Password) < minPasswordLen:
		add("admin.password", "Password must be at least 8 characters long")
	case len(req.Admin.Password) > maxPasswordLen:
		add("admin.password", "Password must be at most 72 characters long")
	default:
		in.AdminPassword = req.Admin.Password
	}

	return in, issues
}

// parseOwnerAndStudio holds the rules shared by onboarding and editing. The domain name is only
// checked when checkDomain is true (editing locks it).
func parseOwnerAndStudio(req StudioRequest, checkDomain bool) (StudioInput, []FieldIssue) {
	var issues []FieldIssue
	add := func(path, msg string) { issues = append(issues, FieldIssue{Path: path, Message: msg}) }

	required := func(path, label, value string, max int) string {
		value = strings.TrimSpace(value)
		switch {
		case value == "":
			add(path, label+" is required")
		case len(value) > max:
			add(path, label+" must be at most "+strconv.Itoa(max)+" characters")
		}
		return value
	}

	in := StudioInput{}

	// owner
	in.OwnerName = required("owner.full_name", "Owner full name", req.Owner.FullName, 100)

	if mobile := strings.TrimSpace(req.Owner.Mobile); mobile == "" {
		add("owner.mobile", "Mobile number is required")
	} else if cleaned, ok := cleanMobile(mobile); !ok {
		add("owner.mobile", "Mobile number must be 10-15 digits, optionally starting with +")
	} else {
		in.OwnerMobile = cleaned
	}

	if email := strings.ToLower(strings.TrimSpace(req.Owner.BusinessEmail)); email == "" {
		add("owner.business_email", "Business email is required")
	} else if !validBusinessEmail(email) || len(email) > 150 {
		add("owner.business_email", "Business email is not a valid email address")
	} else {
		in.BusinessEmail = email
	}

	// studio
	in.StudioName = required("studio.name", "Studio name", req.Studio.Name, 150)

	if checkDomain {
		in.Domain = NormalizeDomainName(req.Studio.Domain)
		switch {
		case in.Domain == "":
			add("studio.domain", "Studio domain is required")
		case !ValidDomainName(in.Domain):
			add("studio.domain", "Domain must be 3-50 characters: lowercase letters, digits and hyphens only")
		case IsReservedDomainName(in.Domain):
			add("studio.domain", "This domain name is reserved")
		}
	}

	in.Address = required("studio.address", "Address", req.Studio.Address, 255)
	in.City = required("studio.city", "City", req.Studio.City, 100)
	in.State = required("studio.state", "State", req.Studio.State, 100)

	if pin := strings.TrimSpace(req.Studio.Pincode); pin == "" {
		add("studio.pincode", "Pincode is required")
	} else if !pincodePattern.MatchString(pin) {
		add("studio.pincode", "Pincode must be 6 digits")
	} else {
		in.Pincode = pin
	}

	if wa := strings.TrimSpace(req.Studio.Whatsapp); wa != "" {
		if cleaned, ok := cleanMobile(wa); ok {
			in.Whatsapp = &cleaned
		} else {
			add("studio.whatsapp", "WhatsApp number must be 10-15 digits, optionally starting with +")
		}
	}

	if gst := strings.ToUpper(strings.TrimSpace(req.Studio.GSTIN)); gst != "" {
		if gstinPattern.MatchString(gst) {
			in.GSTIN = &gst
		} else {
			add("studio.gstin", "GSTIN must be a valid 15-character GSTIN")
		}
	}

	if site, ok := cleanURL(req.Studio.Website); !ok {
		add("studio.website", "Website must be a valid URL, e.g. https://example.com")
	} else if site != "" {
		in.Website = &site
	}

	in.Instagram = cleanSocial(&issues, "studio.instagram", "Instagram", req.Studio.Instagram, instagramRe)
	in.Facebook = cleanSocial(&issues, "studio.facebook", "Facebook", req.Studio.Facebook, otherSocialRe)
	in.YouTube = cleanSocial(&issues, "studio.youtube", "YouTube", req.Studio.YouTube, otherSocialRe)

	return in, issues
}

func cleanMobile(raw string) (string, bool) {
	cleaned := mobileCleaner.Replace(strings.TrimSpace(raw))
	return cleaned, mobilePattern.MatchString(cleaned)
}

func validBusinessEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	at := strings.LastIndex(email, "@")
	return at > 0 && strings.Contains(email[at+1:], ".")
}

// cleanURL returns ("", true) for a blank value (including the placeholder "https://"), the cleaned
// URL for a valid one, and false for an invalid one. A missing scheme defaults to https.
func cleanURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "", "http://", "https://":
		return "", true
	}
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || !strings.Contains(u.Host, ".") || len(raw) > 255 {
		return "", false
	}
	return raw, true
}

// cleanSocial accepts a profile URL or an @handle; blank means "not given".
func cleanSocial(issues *[]FieldIssue, path, label, raw string, handle *regexp.Regexp) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.Contains(raw, "/") {
		if cleaned, ok := cleanURL(raw); ok && cleaned != "" {
			return &cleaned
		}
		*issues = append(*issues, FieldIssue{Path: path, Message: label + " must be a valid URL or an @handle"})
		return nil
	}
	if !handle.MatchString(raw) {
		*issues = append(*issues, FieldIssue{Path: path, Message: label + " must be a valid URL or an @handle"})
		return nil
	}
	if !strings.HasPrefix(raw, "@") {
		raw = "@" + raw
	}
	return &raw
}
