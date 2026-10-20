package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// catalog: eventManagment > eventBoard > {read, edit}; eventManagment > eventList > {view}; clientManagment (empty)
func catalog() []Permission {
	g, f1, f2 := uint64(1), uint64(2), uint64(3)
	return []Permission{
		{ID: 1, Level: PermissionLevelGroup, Name: "eventManagment", Code: "gRe3"},
		{ID: 2, Level: PermissionLevelFeature, Name: "eventBoard", Code: "uhMf", ParentID: &g},
		{ID: 3, Level: PermissionLevelFeature, Name: "eventList", Code: "Lk01", ParentID: &g},
		{ID: 4, Level: PermissionLevelPermission, Name: "read", Code: "vFe3", ParentID: &f1},
		{ID: 5, Level: PermissionLevelPermission, Name: "edit", Code: "Tq8c", ParentID: &f1},
		{ID: 6, Level: PermissionLevelPermission, Name: "view", Code: "Vw02", ParentID: &f2},
		{ID: 7, Level: PermissionLevelGroup, Name: "clientManagment", Code: "k9Qx"},
	}
}

func shareBody(t *testing.T, body string) []FeatureGroupItem {
	t.Helper()
	groups, _, issues := ParsePermissionPayload([]byte(body))
	if len(issues) > 0 {
		t.Fatalf("bad test body: %+v", issues)
	}
	return groups
}

const readAndEdit = `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[
  {"name":"eventBoard","code":"uhMf","permission":[{"name":"read","code":"vFe3"},{"name":"edit","code":"Tq8c"}]}]}}`

func TestShareReplaceAddRemoveClear(t *testing.T) {
	groups := shareBody(t, readAndEdit)
	current := map[uint64]bool{5: true, 6: true} // edit and view already shared

	cases := []struct {
		mode        string
		add, remove []uint64
	}{
		{ShareModeReplace, []uint64{4}, []uint64{6}}, // want read+edit: add read, drop view
		{ShareModeAdd, []uint64{4}, nil},             // keep view, add read (edit already there)
		{ShareModeRemove, nil, []uint64{5}},          // only edit was shared of the two
		{ShareModeClear, nil, []uint64{5, 6}},
	}
	for _, tc := range cases {
		plan, issues := PlanPermissionShare(catalog(), current, tc.mode, groups, false)
		if len(issues) > 0 {
			t.Fatalf("%s: unexpected issues %+v", tc.mode, issues)
		}
		if !reflect.DeepEqual(plan.Add, tc.add) || !reflect.DeepEqual(plan.Remove, tc.remove) {
			t.Errorf("%s: add=%v remove=%v, want add=%v remove=%v", tc.mode, plan.Add, plan.Remove, tc.add, tc.remove)
		}
	}
}

func TestShareClearNeedsNoPermissions(t *testing.T) {
	plan, issues := PlanPermissionShare(catalog(), map[uint64]bool{4: true}, ShareModeClear, nil, false)
	if len(issues) > 0 || !reflect.DeepEqual(plan.Remove, []uint64{4}) {
		t.Fatalf("plan=%+v issues=%+v", plan, issues)
	}
}

func TestShareRejectsBadRequests(t *testing.T) {
	cases := map[string]struct {
		mode     string
		body     string
		contains string
		conflict bool
	}{
		"unknown mode":        {"merge", readAndEdit, "Mode must be one of", false},
		"unknown code":        {ShareModeAdd, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"uhMf","permission":[{"name":"nope","code":"Zz99"}]}]}}`, "does not exist", false},
		"wrong name":          {ShareModeAdd, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"uhMf","permission":[{"name":"READ","code":"vFe3"}]}]}}`, "has the name 'read'", true},
		"wrong parent":        {ShareModeAdd, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"uhMf","permission":[{"name":"view","code":"Vw02"}]}]}}`, "does not belong under its parent", true},
		"wrong level":         {ShareModeAdd, `{"featureGroup":{"name":"eventBoard","code":"uhMf"}}`, "is a feature, not a group", true},
		"nothing to share":    {ShareModeAdd, `{"featureGroup":{"name":"eventManagment","code":"gRe3"}}`, "Send at least one permission", false},
		"empty features only": {ShareModeReplace, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"uhMf"}]}}`, "Send at least one permission", false},
	}
	for label, tc := range cases {
		_, issues := PlanPermissionShare(catalog(), nil, tc.mode, shareBody(t, tc.body), false)
		if len(issues) == 0 {
			t.Errorf("%s: expected an issue", label)
			continue
		}
		if !strings.Contains(issues[0].Message, tc.contains) || issues[0].Conflict != tc.conflict {
			t.Errorf("%s: got %+v, want message containing %q (conflict=%v)", label, issues[0], tc.contains, tc.conflict)
		}
	}
}

func TestShareReportsAllProblemsAtOnce(t *testing.T) {
	body := `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[{"name":"eventBoard","code":"uhMf","permission":[
	  {"name":"a","code":"Zz01"},{"name":"b","code":"Zz02"}]}]}}`
	_, issues := PlanPermissionShare(catalog(), nil, ShareModeAdd, shareBody(t, body), false)
	if len(issues) != 2 || issues[1].Path != "permissions[0].featureGroup.feature[0].permission[1].code" {
		t.Fatalf("expected 2 issues with paths, got %+v", issues)
	}
}

func TestChildDomainSeesOnlySharedPermissions(t *testing.T) {
	// acme was shared only "read" (4) and "view" (6)
	visible := FilterPermissionRows(catalog(), map[uint64]bool{4: true, 6: true})
	tree := BuildPermissionTree(visible)

	raw, _ := json.Marshal(tree)
	want := `[{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[` +
		`{"name":"eventBoard","code":"uhMf","permission":[{"name":"read","code":"vFe3"}]},` +
		`{"name":"eventList","code":"Lk01","permission":[{"name":"view","code":"Vw02"}]}]}}]`
	if string(raw) != want {
		t.Fatalf("\n got %s\nwant %s", raw, want)
	}

	// "edit" was not shared: its code is unknown to acme, and the empty group is hidden entirely
	if BuildPermissionBranch(visible, "Tq8c") != nil || BuildPermissionBranch(visible, "k9Qx") != nil {
		t.Fatalf("unshared codes must not be visible")
	}
	if b := BuildPermissionBranch(visible, "vFe3"); b == nil || len(b.FeatureGroup.Feature) != 1 {
		t.Fatalf("shared permission should be visible, got %+v", b)
	}
}

func TestNothingSharedMeansEmptyTree(t *testing.T) {
	tree := BuildPermissionTree(FilterPermissionRows(catalog(), map[uint64]bool{}))
	raw, _ := json.Marshal(tree)
	if string(raw) != "[]" {
		t.Fatalf("expected [], got %s", raw)
	}
}

func TestShareOnlyPermissionsYouOwn(t *testing.T) {
	// the sharer owns only the "client" group; "gRe3" belongs to some other domain and is not in rows
	owned := []Permission{
		{ID: 10, Level: PermissionLevelGroup, Name: "clientManagment", Code: "k9Qx"},
	}
	_, issues := PlanPermissionShare(owned, nil, ShareModeAdd, shareBody(t, readAndEdit), false)
	if len(issues) == 0 || !strings.Contains(issues[0].Message, "does not exist in your domain's permissions") || issues[0].Conflict {
		t.Fatalf("another domain's code must look unknown, got %+v", issues)
	}
}

func TestReplaceOnlyTouchesTheSharersOwnShares(t *testing.T) {
	// current holds only what THIS sharer already gave the target (permission 5); anything the target
	// received from other domains is not in current, so replace can never remove it.
	groups := shareBody(t, `{"featureGroup":{"name":"eventManagment","code":"gRe3","feature":[
	  {"name":"eventBoard","code":"uhMf","permission":[{"name":"read","code":"vFe3"}]}]}}`)
	plan, issues := PlanPermissionShare(catalog(), map[uint64]bool{5: true}, ShareModeReplace, groups, false)
	if len(issues) > 0 || !reflect.DeepEqual(plan.Add, []uint64{4}) || !reflect.DeepEqual(plan.Remove, []uint64{5}) {
		t.Fatalf("plan=%+v issues=%+v", plan, issues)
	}
}
