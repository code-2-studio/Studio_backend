package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The JSON shape used to send and receive permissions:
//
//	{ "featureGroup": { "name": "...", "code": "...",
//	    "feature": [ { "name": "...", "code": "...",
//	        "permission": [ { "name": "...", "code": "..." } ] } ] } }

type PermissionItem struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type FeatureItem struct {
	Name       string           `json:"name"`
	Code       string           `json:"code"`
	Permission []PermissionItem `json:"permission"`
}

type FeatureGroupItem struct {
	Name    string        `json:"name"`
	Code    string        `json:"code"`
	Feature []FeatureItem `json:"feature"`
}

type FeatureGroupEnvelope struct {
	FeatureGroup FeatureGroupItem `json:"featureGroup"`
}

// PermissionIssue is one problem found in a request, located by its JSON path.
type PermissionIssue struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Conflict bool   `json:"-"` // clashes with existing data (409) rather than a bad value (422)
}

// PlannedPermission is a node that has to be created. Parents come before their children.
type PlannedPermission struct {
	Level      string
	Name       string
	Code       string
	ParentCode string // empty for a group
}

var permissionCodePattern = regexp.MustCompile(`^[A-Za-z0-9]{2,32}$`)

const permissionNameMaxLen = 100

// ValidPermissionCode reports whether a code is 2-32 letters or digits.
func ValidPermissionCode(code string) bool {
	return permissionCodePattern.MatchString(code)
}

// ValidPermissionName reports whether an already-trimmed name is usable.
func ValidPermissionName(name string) bool {
	return name != "" && len(name) <= permissionNameMaxLen
}

// ParsePermissionPayload reads the request body: one {"featureGroup": ...} object or an array of them.
// Unknown keys are rejected so a typo such as "features" cannot be silently dropped.
func ParsePermissionPayload(body []byte) (groups []FeatureGroupItem, isArray bool, issues []PermissionIssue) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, false, []PermissionIssue{{Path: "", Message: "Request body is required"}}
	}

	var envelopes []FeatureGroupEnvelope
	isArray = trimmed[0] == '['

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var err error
	if isArray {
		err = dec.Decode(&envelopes)
	} else {
		var single FeatureGroupEnvelope
		if err = dec.Decode(&single); err == nil {
			envelopes = []FeatureGroupEnvelope{single}
		}
	}
	if err == nil && dec.More() {
		err = fmt.Errorf("unexpected data after the JSON value")
	}
	if err != nil {
		return nil, isArray, []PermissionIssue{{Path: "", Message: "Invalid JSON: " + err.Error()}}
	}
	if len(envelopes) == 0 {
		return nil, isArray, []PermissionIssue{{Path: "", Message: "At least one featureGroup is required"}}
	}

	groups = make([]FeatureGroupItem, 0, len(envelopes))
	for _, env := range envelopes {
		groups = append(groups, normalizeGroup(env.FeatureGroup))
	}
	return groups, isArray, ValidatePermissionGroups(groups, isArray)
}

// normalizeGroup trims names and codes and replaces missing lists with empty ones.
func normalizeGroup(g FeatureGroupItem) FeatureGroupItem {
	g.Name, g.Code = strings.TrimSpace(g.Name), strings.TrimSpace(g.Code)
	if g.Feature == nil {
		g.Feature = []FeatureItem{}
	}
	for i := range g.Feature {
		f := &g.Feature[i]
		f.Name, f.Code = strings.TrimSpace(f.Name), strings.TrimSpace(f.Code)
		if f.Permission == nil {
			f.Permission = []PermissionItem{}
		}
		for j := range f.Permission {
			p := &f.Permission[j]
			p.Name, p.Code = strings.TrimSpace(p.Name), strings.TrimSpace(p.Code)
		}
	}
	return g
}

// ValidatePermissionGroups checks names, codes and duplicates inside the request itself.
// Every level needs both a name and a code.
func ValidatePermissionGroups(groups []FeatureGroupItem, isArray bool) []PermissionIssue {
	var issues []PermissionIssue
	seenCodes := map[string]string{} // code -> path where first seen

	check := func(path, name, code string) {
		if !ValidPermissionName(name) {
			issues = append(issues, PermissionIssue{Path: path + ".name", Message: "Name is required (max 100 characters)"})
		}
		switch {
		case code == "":
			issues = append(issues, PermissionIssue{Path: path + ".code", Message: "Code is required"})
		case !ValidPermissionCode(code):
			issues = append(issues, PermissionIssue{Path: path + ".code", Message: "Code must be 2-32 letters or digits"})
		default:
			if first, dup := seenCodes[code]; dup {
				issues = append(issues, PermissionIssue{Path: path + ".code", Message: fmt.Sprintf("Code '%s' is already used at %s", code, first)})
			} else {
				seenCodes[code] = path
			}
		}
	}

	checkSiblingNames := func(path string, names []string) {
		seen := map[string]bool{}
		for i, n := range names {
			if n == "" {
				continue
			}
			if seen[n] {
				issues = append(issues, PermissionIssue{Path: fmt.Sprintf("%s[%d].name", path, i), Message: fmt.Sprintf("Name '%s' appears more than once at this level", n)})
			}
			seen[n] = true
		}
	}

	groupNames := make([]string, len(groups))
	for gi, g := range groups {
		groupNames[gi] = g.Name
		gp := "featureGroup"
		if isArray {
			gp = fmt.Sprintf("[%d].featureGroup", gi)
		}
		check(gp, g.Name, g.Code)

		featureNames := make([]string, len(g.Feature))
		for fi, f := range g.Feature {
			featureNames[fi] = f.Name
			fp := fmt.Sprintf("%s.feature[%d]", gp, fi)
			check(fp, f.Name, f.Code)

			permNames := make([]string, len(f.Permission))
			for pi, p := range f.Permission {
				permNames[pi] = p.Name
				check(fmt.Sprintf("%s.permission[%d]", fp, pi), p.Name, p.Code)
			}
			checkSiblingNames(fp+".permission", permNames)
		}
		checkSiblingNames(gp+".feature", featureNames)
	}
	seenGroup := map[string]bool{}
	for gi, n := range groupNames {
		if n != "" && seenGroup[n] {
			issues = append(issues, PermissionIssue{Path: fmt.Sprintf("[%d].featureGroup.name", gi), Message: fmt.Sprintf("Name '%s' appears more than once at this level", n)})
		}
		seenGroup[n] = true
	}
	return issues
}

// PlanPermissionSave compares a validated request with the stored catalog and works out what to create.
//
// Nodes are matched by code. A code that already exists in the same place is left alone. A code that
// exists with another level or parent, a different name, or a name that is already taken by a
// sibling is reported as a conflict, and then nothing should be saved.
//
// Permissions belong to the domain that created them (ownerDomainID). A code owned by another domain
// is simply "in use", and names only have to be unique among the owner's own siblings.
func PlanPermissionSave(existing []Permission, ownerDomainID uint64, groups []FeatureGroupItem, isArray bool) (create []PlannedPermission, existingCount int, issues []PermissionIssue) {
	byCode := make(map[string]*Permission, len(existing))
	siblings := map[uint64]map[string]*Permission{} // parent id (0 = top level) -> name -> node (owner's nodes only)
	for i := range existing {
		n := &existing[i]
		byCode[n.Code] = n
		if n.DomainID != ownerDomainID {
			continue
		}
		parent := uint64(0)
		if n.ParentID != nil {
			parent = *n.ParentID
		}
		if siblings[parent] == nil {
			siblings[parent] = map[string]*Permission{}
		}
		siblings[parent][n.Name] = n
	}

	conflict := func(path, msg string) {
		issues = append(issues, PermissionIssue{Path: path, Message: msg, Conflict: true})
	}

	// visit handles one node. parent is the stored parent (nil for a top-level group or when the
	// parent is itself new). It returns the stored node if there is one and it is safe to descend into.
	visit := func(level, path, name, code string, parent *Permission, parentIsNew bool, parentCode string) (node *Permission, isNew bool, ok bool) {
		if found := byCode[code]; found != nil {
			switch {
			case found.DomainID != ownerDomainID:
				conflict(path+".code", fmt.Sprintf("Code '%s' is already in use", code))
				return nil, false, false
			case found.Level != level:
				conflict(path+".code", fmt.Sprintf("Code '%s' already exists as a %s", code, found.Level))
				return nil, false, false
			case level != PermissionLevelGroup && (parentIsNew || parent == nil || found.ParentID == nil || *found.ParentID != parent.ID):
				conflict(path+".code", fmt.Sprintf("Code '%s' already exists under a different parent", code))
				return nil, false, false
			case found.Name != name:
				conflict(path+".code", fmt.Sprintf("Code '%s' already exists with the name '%s'; use the update API to rename it", code, found.Name))
				return nil, false, false
			}
			existingCount++
			return found, false, true
		}

		if !parentIsNew {
			parentID := uint64(0)
			if parent != nil {
				parentID = parent.ID
			}
			if clash := siblings[parentID][name]; clash != nil {
				conflict(path+".name", fmt.Sprintf("A %s named '%s' already exists here with code '%s'", level, name, clash.Code))
				return nil, false, false
			}
		}
		create = append(create, PlannedPermission{Level: level, Name: name, Code: code, ParentCode: parentCode})
		return nil, true, true
	}

	for gi, g := range groups {
		gp := "featureGroup"
		if isArray {
			gp = fmt.Sprintf("[%d].featureGroup", gi)
		}
		gNode, gNew, ok := visit(PermissionLevelGroup, gp, g.Name, g.Code, nil, false, "")
		if !ok {
			continue
		}

		for fi, f := range g.Feature {
			fp := fmt.Sprintf("%s.feature[%d]", gp, fi)
			fNode, fNew, ok := visit(PermissionLevelFeature, fp, f.Name, f.Code, gNode, gNew, g.Code)
			if !ok {
				continue
			}
			for pi, p := range f.Permission {
				visit(PermissionLevelPermission, fmt.Sprintf("%s.permission[%d]", fp, pi), p.Name, p.Code, fNode, fNew, f.Code)
			}
		}
	}
	return create, existingCount, issues
}

// BuildPermissionTree turns stored nodes into the JSON shape, in creation order.
func BuildPermissionTree(rows []Permission) []FeatureGroupEnvelope {
	return buildTree(rows, nil)
}

// BuildPermissionBranch returns the group containing the node with the given code. For a group it holds
// the whole group; for a feature, that feature with its permissions; for a permission, just that one.
// It returns nil when the code does not exist.
func BuildPermissionBranch(rows []Permission, code string) *FeatureGroupEnvelope {
	var target *Permission
	for i := range rows {
		if rows[i].Code == code {
			target = &rows[i]
			break
		}
	}
	if target == nil {
		return nil
	}

	only := map[uint64]bool{target.ID: true}
	// keep everything below the target (before adding ancestors, so siblings are not pulled in)
	for changed := true; changed; {
		changed = false
		for i := range rows {
			if rows[i].ParentID != nil && only[*rows[i].ParentID] && !only[rows[i].ID] {
				only[rows[i].ID] = true
				changed = true
			}
		}
	}
	// keep every ancestor of the target
	byID := make(map[uint64]*Permission, len(rows))
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}
	for cur := target; cur.ParentID != nil; {
		parent := byID[*cur.ParentID]
		if parent == nil {
			break
		}
		only[parent.ID] = true
		cur = parent
	}

	trees := buildTree(rows, only)
	if len(trees) == 0 {
		return nil
	}
	return &trees[0]
}

func buildTree(rows []Permission, only map[uint64]bool) []FeatureGroupEnvelope {
	keep := func(p *Permission) bool { return only == nil || only[p.ID] }

	featuresOf := map[uint64][]*Permission{}
	permissionsOf := map[uint64][]*Permission{}
	var groups []*Permission
	for i := range rows {
		n := &rows[i]
		if !keep(n) {
			continue
		}
		switch n.Level {
		case PermissionLevelGroup:
			groups = append(groups, n)
		case PermissionLevelFeature:
			if n.ParentID != nil {
				featuresOf[*n.ParentID] = append(featuresOf[*n.ParentID], n)
			}
		case PermissionLevelPermission:
			if n.ParentID != nil {
				permissionsOf[*n.ParentID] = append(permissionsOf[*n.ParentID], n)
			}
		}
	}

	out := make([]FeatureGroupEnvelope, 0, len(groups))
	for _, g := range groups {
		item := FeatureGroupItem{Name: g.Name, Code: g.Code, Feature: []FeatureItem{}}
		for _, f := range featuresOf[g.ID] {
			feature := FeatureItem{Name: f.Name, Code: f.Code, Permission: []PermissionItem{}}
			for _, p := range permissionsOf[f.ID] {
				feature.Permission = append(feature.Permission, PermissionItem{Name: p.Name, Code: p.Code})
			}
			item.Feature = append(item.Feature, feature)
		}
		out = append(out, FeatureGroupEnvelope{FeatureGroup: item})
	}
	return out
}

// FilterPermissionRows keeps only the permission-level nodes in allowed, plus the groups and features
// above them. This is what a child domain is allowed to see.
func FilterPermissionRows(rows []Permission, allowed map[uint64]bool) []Permission {
	byID := make(map[uint64]*Permission, len(rows))
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}

	keep := map[uint64]bool{}
	for i := range rows {
		n := &rows[i]
		if n.Level != PermissionLevelPermission || !allowed[n.ID] {
			continue
		}
		for cur := n; cur != nil; {
			keep[cur.ID] = true
			if cur.ParentID == nil {
				break
			}
			cur = byID[*cur.ParentID]
		}
	}

	out := make([]Permission, 0, len(keep))
	for _, r := range rows {
		if keep[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// SharePlan is the change a share request makes to a domain's permission set (permission ids).
type SharePlan struct {
	Add    []uint64
	Remove []uint64
}

// PlanPermissionShare checks a share request against the sharer's own permissions and works out what to
// add and remove. rows must contain only permissions the sharer owns, and current only the ids the sharer
// has already shared with the target domain.
//
// Every code must exist in the catalog with the same name, level and parent; otherwise issues are
// returned and nothing should be changed. Only permission-level entries grant access: a group or
// feature listed without permissions inside shares nothing.
func PlanPermissionShare(rows []Permission, current map[uint64]bool, mode string, groups []FeatureGroupItem, isArray bool) (SharePlan, []PermissionIssue) {
	var issues []PermissionIssue

	switch mode {
	case ShareModeReplace, ShareModeAdd, ShareModeRemove, ShareModeClear:
	default:
		return SharePlan{}, []PermissionIssue{{Path: "mode", Message: "Mode must be one of: replace, add, remove, clear"}}
	}

	requested := map[uint64]bool{}
	if mode != ShareModeClear {
		byCode := make(map[string]*Permission, len(rows))
		for i := range rows {
			byCode[rows[i].Code] = &rows[i]
		}

		// check returns the stored node when the entry matches the catalog exactly
		check := func(level, path, name, code string, parent *Permission) *Permission {
			node := byCode[code]
			switch {
			case node == nil:
				issues = append(issues, PermissionIssue{Path: path + ".code", Message: fmt.Sprintf("Code '%s' does not exist in your domain's permissions", code)})
				return nil
			case node.Level != level:
				issues = append(issues, PermissionIssue{Path: path + ".code", Message: fmt.Sprintf("Code '%s' is a %s, not a %s", code, node.Level, level), Conflict: true})
				return nil
			case node.Name != name:
				issues = append(issues, PermissionIssue{Path: path + ".name", Message: fmt.Sprintf("Code '%s' has the name '%s', not '%s'", code, node.Name, name), Conflict: true})
				return nil
			case parent != nil && (node.ParentID == nil || *node.ParentID != parent.ID):
				issues = append(issues, PermissionIssue{Path: path + ".code", Message: fmt.Sprintf("Code '%s' does not belong under its parent in this request", code), Conflict: true})
				return nil
			}
			return node
		}

		for gi, g := range groups {
			gp := "permissions[0].featureGroup"
			if isArray {
				gp = fmt.Sprintf("permissions[%d].featureGroup", gi)
			}
			gNode := check(PermissionLevelGroup, gp, g.Name, g.Code, nil)
			if gNode == nil {
				continue
			}
			for fi, f := range g.Feature {
				fp := fmt.Sprintf("%s.feature[%d]", gp, fi)
				fNode := check(PermissionLevelFeature, fp, f.Name, f.Code, gNode)
				if fNode == nil {
					continue
				}
				for pi, p := range f.Permission {
					if pNode := check(PermissionLevelPermission, fmt.Sprintf("%s.permission[%d]", fp, pi), p.Name, p.Code, fNode); pNode != nil {
						requested[pNode.ID] = true
					}
				}
			}
		}

		if len(issues) == 0 && len(requested) == 0 {
			issues = append(issues, PermissionIssue{Path: "permissions", Message: "Send at least one permission"})
		}
		if len(issues) > 0 {
			return SharePlan{}, issues
		}
	}

	var plan SharePlan
	switch mode {
	case ShareModeReplace:
		for id := range requested {
			if !current[id] {
				plan.Add = append(plan.Add, id)
			}
		}
		for id := range current {
			if !requested[id] {
				plan.Remove = append(plan.Remove, id)
			}
		}
	case ShareModeAdd:
		for id := range requested {
			if !current[id] {
				plan.Add = append(plan.Add, id)
			}
		}
	case ShareModeRemove:
		for id := range requested {
			if current[id] {
				plan.Remove = append(plan.Remove, id)
			}
		}
	case ShareModeClear:
		for id := range current {
			plan.Remove = append(plan.Remove, id)
		}
	}
	sort.Slice(plan.Add, func(i, j int) bool { return plan.Add[i] < plan.Add[j] })
	sort.Slice(plan.Remove, func(i, j int) bool { return plan.Remove[i] < plan.Remove[j] })
	return plan, nil
}
