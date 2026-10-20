package models

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleTree = `{
  "featureGroup": {
    "name": "eventManagment",
    "code": "gRe3",
    "feature": [
      { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" } ] }
    ]
  }
}`

// stored mirrors what the database would hold after sampleTree was saved.
func stored() []Permission {
	one, two := uint64(1), uint64(2)
	return []Permission{
		{ID: 1, Level: PermissionLevelGroup, Name: "eventManagment", Code: "gRe3"},
		{ID: 2, Level: PermissionLevelFeature, Name: "eventBoard", Code: "uhMf", ParentID: &one},
		{ID: 3, Level: PermissionLevelPermission, Name: "read", Code: "vFe3", ParentID: &two},
	}
}

func mustParse(t *testing.T, body string) ([]FeatureGroupItem, bool) {
	t.Helper()
	groups, isArray, issues := ParsePermissionPayload([]byte(body))
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	return groups, isArray
}

func TestParseAcceptsYourShape(t *testing.T) {
	groups, isArray := mustParse(t, sampleTree)
	if isArray || len(groups) != 1 || groups[0].Code != "gRe3" ||
		groups[0].Feature[0].Code != "uhMf" || groups[0].Feature[0].Permission[0].Code != "vFe3" {
		t.Fatalf("parsed wrongly: %+v", groups)
	}
}

func TestParseAllowsPartialLevelsAndArray(t *testing.T) {
	groups, isArray := mustParse(t, `[
	  {"featureGroup": {"name": "clientManagment", "code": "k9Qx"}},
	  {"featureGroup": {"name": "x", "code": "ab", "feature": [{"name": "f", "code": "cd"}]}}
	]`)
	if !isArray || len(groups) != 2 {
		t.Fatalf("expected 2 groups from array, got %+v", groups)
	}
	if groups[0].Feature == nil || len(groups[0].Feature) != 0 {
		t.Fatalf("missing feature list should become an empty list")
	}
	if groups[1].Feature[0].Permission == nil {
		t.Fatalf("missing permission list should become an empty list")
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		body string
		path string
	}{
		"empty body":     {``, ""},
		"bad json":       {`{`, ""},
		"unknown key":    {`{"featureGroup":{"name":"a","code":"ab","features":[]}}`, ""},
		"missing code":   {`{"featureGroup":{"name":"a"}}`, "featureGroup.code"},
		"short code":     {`{"featureGroup":{"name":"a","code":"x"}}`, "featureGroup.code"},
		"symbol code":    {`{"featureGroup":{"name":"a","code":"a-b"}}`, "featureGroup.code"},
		"missing name":   {`{"featureGroup":{"code":"ab"}}`, "featureGroup.name"},
		"nested no code": {`{"featureGroup":{"name":"a","code":"ab","feature":[{"name":"f","code":"cd","permission":[{"name":"p"}]}]}}`, "featureGroup.feature[0].permission[0].code"},
	}
	for label, tc := range cases {
		_, _, issues := ParsePermissionPayload([]byte(tc.body))
		if len(issues) == 0 {
			t.Errorf("%s: expected an issue", label)
			continue
		}
		if tc.path != "" && issues[0].Path != tc.path {
			t.Errorf("%s: path = %q, want %q", label, issues[0].Path, tc.path)
		}
	}
}

func TestParseRejectsDuplicatesInsideRequest(t *testing.T) {
	_, _, issues := ParsePermissionPayload([]byte(`{"featureGroup":{"name":"a","code":"ab","feature":[
	  {"name":"f1","code":"cd"},{"name":"f2","code":"cd"},{"name":"f1","code":"ef"}]}}`))
	var dupCode, dupName bool
	for _, is := range issues {
		dupCode = dupCode || strings.Contains(is.Message, "already used")
		dupName = dupName || strings.Contains(is.Message, "appears more than once")
	}
	if !dupCode || !dupName {
		t.Fatalf("expected duplicate code and name issues, got %+v", issues)
	}
}

func TestPlanCreatesEverythingOnEmptyCatalog(t *testing.T) {
	groups, isArray := mustParse(t, sampleTree)
	create, existing, issues := PlanPermissionSave(nil, 0, groups, isArray)
	if len(issues) > 0 || existing != 0 || len(create) != 3 {
		t.Fatalf("create=%d existing=%d issues=%+v", len(create), existing, issues)
	}
	// parents must come before children
	if create[0].Level != PermissionLevelGroup || create[1].ParentCode != "gRe3" || create[2].ParentCode != "uhMf" {
		t.Fatalf("bad order/parents: %+v", create)
	}
}

func TestPlanIsIdempotent(t *testing.T) {
	groups, isArray := mustParse(t, sampleTree)
	create, existing, issues := PlanPermissionSave(stored(), 0, groups, isArray)
	if len(issues) > 0 || len(create) != 0 || existing != 3 {
		t.Fatalf("create=%d existing=%d issues=%+v", len(create), existing, issues)
	}
}

func TestPlanAddsNewPermissionUnderExistingParents(t *testing.T) {
	groups, isArray := mustParse(t, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[
	  {"name":"eventBoard","code":"uhMf","permission":[{"name":"write","code":"Tq7b"}]}]}}`)
	create, existing, issues := PlanPermissionSave(stored(), 0, groups, isArray)
	if len(issues) > 0 || existing != 2 || len(create) != 1 ||
		create[0].Code != "Tq7b" || create[0].ParentCode != "uhMf" {
		t.Fatalf("create=%+v existing=%d issues=%+v", create, existing, issues)
	}
}

func TestPlanReportsConflicts(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"code under a different parent": {
			`{"featureGroup":{"name":"other","code":"zz11","feature":[{"name":"eventBoard","code":"uhMf"}]}}`,
			"different parent"},
		"code at another level": {
			`{"featureGroup":{"name":"x","code":"uhMf"}}`,
			"already exists as a feature"},
		"same code, different name": {
			`{"featureGroup":{"name":"renamed","code":"gRe3"}}`,
			"use the update API"},
		"same name, different code": {
			`{"featureGroup":{"name":"eventManagment","code":"zz22"}}`,
			"already exists here with code 'gRe3'"},
		"sibling name taken": {
			`{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"nw99"}]}}`,
			"already exists here with code 'uhMf'"},
	}
	for label, tc := range cases {
		groups, isArray := mustParse(t, tc.body)
		create, _, issues := PlanPermissionSave(stored(), 0, groups, isArray)
		if len(issues) == 0 {
			t.Errorf("%s: expected a conflict (create=%+v)", label, create)
			continue
		}
		if !issues[0].Conflict || !strings.Contains(issues[0].Message, tc.want) {
			t.Errorf("%s: got %+v, want message containing %q", label, issues[0], tc.want)
		}
	}
}

func TestTreeAndBranchOutputShape(t *testing.T) {
	one, two := uint64(1), uint64(2)
	rows := append(stored(),
		Permission{ID: 4, Level: PermissionLevelPermission, Name: "write", Code: "Tq7b", ParentID: &two},
		Permission{ID: 5, Level: PermissionLevelGroup, Name: "clientManagment", Code: "k9Qx"},
	)
	_ = one

	tree := BuildPermissionTree(rows)
	if len(tree) != 2 || len(tree[0].FeatureGroup.Feature[0].Permission) != 2 {
		t.Fatalf("tree wrong: %+v", tree)
	}
	raw, _ := json.Marshal(tree[1])
	if string(raw) != `{"featureGroup":{"name":"clientManagment","code":"k9Qx","feature":[]}}` {
		t.Fatalf("empty group should serialise with an empty feature list, got %s", raw)
	}

	// a permission code returns only its own branch
	branch := BuildPermissionBranch(rows, "Tq7b")
	if branch == nil || len(branch.FeatureGroup.Feature) != 1 ||
		len(branch.FeatureGroup.Feature[0].Permission) != 1 ||
		branch.FeatureGroup.Feature[0].Permission[0].Code != "Tq7b" {
		t.Fatalf("branch wrong: %+v", branch)
	}
	// a group code returns the whole group
	whole := BuildPermissionBranch(rows, "gRe3")
	if whole == nil || len(whole.FeatureGroup.Feature[0].Permission) != 2 {
		t.Fatalf("group branch wrong: %+v", whole)
	}
	if BuildPermissionBranch(rows, "nope") != nil {
		t.Fatalf("unknown code should give nil")
	}
}

func TestRoleViewOfPermissionHasOnlyNameAndCode(t *testing.T) {
	raw, _ := json.Marshal(Permission{ID: 9, DomainID: 1, Level: PermissionLevelPermission, Name: "read", Code: "vFe3"})
	if string(raw) != `{"name":"read","code":"vFe3"}` {
		t.Fatalf("permission must expose only name and code, got %s", raw)
	}
}

// storedBy returns the stored catalog with every node owned by the given domain.
func storedBy(domainID uint64) []Permission {
	rows := stored()
	for i := range rows {
		rows[i].DomainID = domainID
	}
	return rows
}

func TestAnotherDomainCannotUseOrExtendYourCodes(t *testing.T) {
	// the tree belongs to domain 1; domain 2 tries to reuse its codes
	groups, isArray := mustParse(t, sampleTree)
	create, _, issues := PlanPermissionSave(storedBy(1), 2, groups, isArray)
	if len(create) != 0 || len(issues) != 1 || !issues[0].Conflict ||
		issues[0].Message != "Code 'gRe3' is already in use" {
		t.Fatalf("create=%+v issues=%+v", create, issues)
	}

	// adding a feature under another domain's group is refused the same way
	groups, isArray = mustParse(t, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"new","code":"nw01"}]}}`)
	if _, _, issues := PlanPermissionSave(storedBy(1), 2, groups, isArray); len(issues) == 0 {
		t.Fatalf("must not be able to extend another domain's group")
	}
}

func TestSameNamesAreFineInDifferentDomains(t *testing.T) {
	// domain 2 makes its own "eventManagment" group with different codes
	groups, isArray := mustParse(t, `{"featureGroup":{"name":"eventManagment","code":"zz01","feature":[
	  {"name":"eventBoard","code":"zz02","permission":[{"name":"read","code":"zz03"}]}]}}`)
	create, existing, issues := PlanPermissionSave(storedBy(1), 2, groups, isArray)
	if len(issues) > 0 || existing != 0 || len(create) != 3 {
		t.Fatalf("create=%d existing=%d issues=%+v", len(create), existing, issues)
	}
}

func TestOwnerStillGetsIdempotentAndNameClashChecks(t *testing.T) {
	groups, isArray := mustParse(t, sampleTree)
	if create, existing, issues := PlanPermissionSave(storedBy(1), 1, groups, isArray); len(issues) > 0 || len(create) != 0 || existing != 3 {
		t.Fatalf("owner resend should be idempotent: create=%d existing=%d issues=%+v", len(create), existing, issues)
	}
	clash, _ := mustParse(t, `{"featureGroup":{"name":"eventManagment","code":"zz22"}}`)
	if _, _, issues := PlanPermissionSave(storedBy(1), 1, clash, false); len(issues) == 0 || !strings.Contains(issues[0].Message, "already exists here") {
		t.Fatalf("owner must not duplicate its own group name: %+v", issues)
	}
}

func TestPermissionCodesAreFlatListsByLevel(t *testing.T) {
	// acme was given only "read" (4) and "view" (6); the group and both features above them come along
	visible := FilterPermissionRows(catalog(), map[uint64]bool{4: true, 6: true})
	codes := PermissionCodes(visible)

	raw, _ := json.Marshal(codes)
	want := `{"featureGroup":["gRe3"],"feature":["uhMf","Lk01"],"permission":["vFe3","Vw02"]}`
	if string(raw) != want {
		t.Fatalf("\n got %s\nwant %s", raw, want)
	}
}

func TestPermissionCodesAreEmptyListsNotNull(t *testing.T) {
	raw, _ := json.Marshal(PermissionCodes(FilterPermissionRows(catalog(), map[uint64]bool{})))
	if string(raw) != `{"featureGroup":[],"feature":[],"permission":[]}` {
		t.Fatalf("empty lists must serialise as [], got %s", raw)
	}
}
