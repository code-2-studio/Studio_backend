# Permissions API Contract

Base URL: `http://<host>:<port>/live/api/v1`

Every parameter goes in the **JSON body**. Nothing is sent in the URL or query string.

> Written from the code. The parsing, validation, sharing and tree-building logic is unit-tested; the HTTP
> endpoints have not been run against a live database yet.

## Common

**Headers (every call)**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```
Send the access token (1 hour), not the refresh token.

**Response envelope**
```json
{ "status": true, "code": 200, "message": "…", "data": … }
```

**Failures**

| Code | Meaning |
|---|---|
| 401 | Not signed in. `reason`: `token_expired` (refresh and retry), `missing_token`, `bad_authorization_header`, `wrong_token_type`, `token_invalid`, `token_revoked`, `user_not_found` |
| 403 | Not allowed (editing is only for permissions your own domain created) |
| 404 | Not found, including a code the caller's domain cannot see |
| 409 | Clashes with stored data |
| 422 | A value is missing or invalid |

Validation and conflict responses list every problem; nothing is saved if there are any:
```json
{ "status": false, "code": 422, "message": "Validation failed, nothing was saved",
  "errors": [ { "path": "featureGroup.feature[0].permission[1].code", "message": "Code must be 2-32 letters or digits" } ] }
```

## The permission tree
```json
{ "featureGroup": {
    "name": "eventManagment", "code": "gRe3",
    "feature": [ { "name": "eventBoard", "code": "uhMf",
      "permission": [ { "name": "read", "code": "vFe3" } ] } ] } }
```
- Three levels: featureGroup, feature, permission. Each has a `name` and a `code`.
- `code`: 2–32 letters or digits, case-sensitive, unique across the catalog. `name`: 1–100 characters.
- Lists are an array of `{ "featureGroup": … }`. `feature` and `permission` are always arrays (`[]` when empty).
- **Anyone signed in can create permissions, whatever their role.** A permission belongs to the domain of the person who created it.
- **Anyone signed in can share, whatever their role.** You share the permissions your domain created with another domain.
- **Visibility:** `shutterx` sees every permission. Any other domain sees the permissions it created plus the ones shared with it.
- Codes are unique across all domains. Two domains may each have a group with the same name, as long as the codes differ.

---

## 1. GET /permissions
What the caller's own domain can see. No body.
- `shutterx`: every permission from every domain.
- Child domain: the permissions it created plus the ones shared with it, with the group and feature above each. A group with nothing visible is hidden.

`200`
```json
{ "status": true, "code": 200, "message": "Permissions retrieved successfully",
  "data": [
    { "featureGroup": { "name": "eventManagment", "code": "gRe3",
        "feature": [ { "name": "eventBoard", "code": "uhMf",
          "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } },
    { "featureGroup": { "name": "clientManagment", "code": "k9Qx", "feature": [] } } ] }
```

## 2. POST /permissions/view
Body is optional; both fields are optional.

| Field | Meaning |
|---|---|
| `code` | Return only the group containing this code |
| `domain` | Shutterx superadmin only: what that domain sees. Another domain's user sending a different domain gets 403. |
| `owned` | `true` returns only the permissions your own domain created. Use it for the share screen, so you only offer what you can share. |
| `shared_with` | A domain name. Returns only your own permissions already shared with that domain, to pre-tick the share screen. Cannot be combined with `owned`. |

`{ "code": "Tq8c" }` → `200`, `data` is one `{ "featureGroup": … }`. A group code returns the whole group, a feature code returns that feature with its permissions, a permission code returns just that one.
```json
{ "status": true, "code": 200, "message": "Permission found",
  "data": { "featureGroup": { "name": "eventManagment", "code": "gRe3",
    "feature": [ { "name": "eventBoard", "code": "uhMf",
      "permission": [ { "name": "edit", "code": "Tq8c" } ] } ] } } }
```
`{ "domain": "acme" }` → `200`, `data` is the array from section 1, as acme sees it.
Unknown or invisible code → `404 "Permission not found"`. Unknown domain → `404 "Domain not found"`.

## 3. POST /permissions   (any signed-in user)
Creates permissions owned by the caller's domain. No role is checked. Send one tree or an array of trees. **Every level needs a `name` and a `code`.**
```json
{ "featureGroup": { "name": "eventManagment", "code": "gRe3",
    "feature": [ { "name": "eventBoard", "code": "uhMf",
      "permission": [ { "name": "read", "code": "vFe3" } ] } ] } }
```
- A group alone is fine: `{ "featureGroup": { "name": "clientManagment", "code": "k9Qx" } }`.
- To add under something your domain already created, send its parents with their existing codes and put the new item inside.
- Items are matched by `code`. An identical existing item of your domain is skipped, never duplicated.
- You cannot reuse a code that another domain owns, and you cannot add items under another domain's group, even one shared with you. Both return `409` with `Code '…' is already in use`.

`201` (nothing new → `200` with `created: 0`; an array input returns `featureGroups: [ {featureGroup}, … ]`)
```json
{ "status": true, "code": 201, "message": "Permissions saved successfully",
  "data": { "created": 3, "existing": 0,
    "featureGroup": { "name": "eventManagment", "code": "gRe3",
      "feature": [ { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" } ] } ] } } }
```
`409` example (a code already exists under another parent, with another name, at another level, or belongs to another domain):
```json
{ "status": false, "code": 409, "message": "Conflicts found, nothing was saved",
  "errors": [ { "path": "featureGroup.feature[0].code", "message": "Code 'uhMf' already exists under a different parent" } ] }
```
`422`: missing or invalid name or code, an unknown key such as `features`, or a duplicate code or name inside the request.
New permissions are automatically given to the shutterx `superadmin` role, whichever domain created them.

## 4. PUT /permissions   (any signed-in user, own domain's permissions only)
Renames an item and/or changes its code. No role is checked, but the item must have been created by the caller's domain.
```json
{ "code": "Tq7b", "name": "edit", "new_code": "Tq8c" }
```
`code` is the item to change. Send `name` and/or `new_code`.
`200`: `data` is the updated branch, with only that item inside.
```json
{ "status": true, "code": 200, "message": "Permission updated successfully",
  "data": { "featureGroup": { "name": "eventManagment", "code": "gRe3",
    "feature": [ { "name": "eventBoard", "code": "uhMf",
      "permission": [ { "name": "edit", "code": "Tq8c" } ] } ] } } }
```
`404` unknown code. `403` the item belongs to another domain (`Forbidden: This permission belongs to another domain`). `409` the new code is taken, or the name is used by another item of your domain at the same level. `422` invalid values.
Roles and sharing are unaffected by a code change, but any frontend code that hard-codes the old code will break.

## 5. PUT /permissions/share   (any signed-in user)
Chooses which of **your domain's own** permissions one other domain can see. Many permissions in one request. No role is checked.
```json
{ "domain": "acme", "mode": "replace", "dry_run": false,
  "permissions": [
    { "featureGroup": { "name": "eventManagment", "code": "gRe3",
      "feature": [ { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } } ] }
```

| Field | Notes |
|---|---|
| `domain` | Required. The domain to share with. Your own domain and `shutterx` are rejected with 422 (shutterx already sees everything). |
| `permissions` | A subset of the tree from `POST /permissions/view` with `{ "owned": true }`, one object or an array. Not needed for `clear`. |
| `mode` | `replace` (default), `add`, `remove`, `clear` |
| `dry_run` | `true` reports what would change and changes nothing |

| `mode` | Effect |
|---|---|
| `replace` | The request becomes the full set **you** share with that domain. Anything you had shared that isn't in the request is removed. Use this for a screen with ticked boxes. |
| `add` | Adds the permissions in the request, keeps the rest |
| `remove` | Removes the permissions in the request, keeps the rest |
| `clear` | Removes everything **you** share with that domain |

Rules:
- You can only share permissions your own domain created. A code from another domain, or one that was shared with you, is reported as `Code '…' does not exist in your domain's permissions`. You cannot pass on what you received.
- `replace`, `remove` and `clear` only touch what you shared. Permissions the target received from other domains are never affected.
- Every `name` and `code` must match your permissions exactly and sit under the right parent. Otherwise `422` or `409` with `errors`, and nothing changes.
- Only permission-level entries grant access. A group or feature sent without permissions shares nothing.
- A permission added to a group later is **not** shared automatically.
- Permissions removed from a domain are also removed from that domain's roles, in the same step.

`200`
```json
{ "status": true, "code": 200, "message": "Permissions shared successfully",
  "data": { "domain": "acme", "mode": "replace", "dry_run": false,
    "added": 2, "removed": 0, "roles_affected": 0,
    "permissions": [ { "featureGroup": { "name": "eventManagment", "code": "gRe3",
      "feature": [ { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } } ] } }
```
`permissions` in the response is what **you** now share with that domain, not everything the domain can see. `roles_affected` is the number of that domain's roles that lost at least one permission. With `dry_run: true` the message is `Dry run, nothing was changed`.

## 6. PUT /roles/permissions
Replaces **all** permissions of one role. Any signed-in user, in their own domain.
```json
{ "role_id": 3, "codes": ["vFe3", "Tq8c"] }
```
- `codes` is required. `[]` removes every permission from the role.
- Only permission-level codes the role's domain can see are accepted: all of them for shutterx; for a child domain, the ones it created plus the ones shared with it.
- `domain` (optional, shutterx superadmin only) acts inside another domain.

`200`: `data` is the role.
```json
{ "status": true, "code": 200, "message": "Role permissions updated successfully",
  "data": { "id": 3, "domain": "acme", "name": "manager", "description": null, "created_at": "…",
    "permissions": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } }
```
`422` with `errors` if a code isn't available, e.g. `{ "path": "codes[1]", "message": "Code 'Zz99' is not available to this domain" }`; nothing changes. `404` if the role isn't in that domain.

## Flows

**Share screen (any domain)**
1. `POST /permissions/view` with `{ "owned": true }` for the permissions your domain can share, to draw the checkbox tree.
2. `POST /permissions/view` with `{ "shared_with": "acme" }` for what you already share with acme, and pre-tick those codes.
3. Build a tree containing only the ticked permissions, each with its feature and group names and codes.
4. Optional: `PUT /permissions/share` with `"dry_run": true` to show `removed` and `roles_affected` in a confirm dialog.
5. Save with `PUT /permissions/share` and `"mode": "replace"`.

**Child domain**
`GET /permissions` shows what the domain created plus what was shared with it. Use it for the role editor, then `PUT /roles/permissions` with the chosen codes. Any user can add their own permissions with `POST /permissions`.
