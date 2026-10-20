# Studio Backend – API Contract

For the frontend. Base URL: `http://<host>:<port>/live/api/v1`

> **Not yet run end to end.** This contract is written from the code. The pure logic (parsing, validation,
> sharing rules, tree building) is unit-tested; the HTTP endpoints and database migrations have not been
> exercised against a live database. Report any difference between this document and real responses.

---

## 1. Conventions

### Request
- JSON bodies, `Content-Type: application/json`.
- **Permission and sharing endpoints take all parameters in the JSON body** – nothing in the URL or query string.
- The user, role and domain endpoints (sections 5–7) still use `/:id` in the path and, for some, `?domain=`; they are listed as they are.

### Response envelope
Every response, success or failure:

```json
{ "status": true, "code": 200, "message": "…", "data": … }
```

Failures have `"status": false`, no `data`, and sometimes an `errors` array (section 2.3).

### Authentication
Every endpoint except register, login, refresh, logout, the first-run setup calls (see [SETUP_API.md](SETUP_API.md)) and the health checks needs:

```
Authorization: Bearer <access_token>
```

(`Authorization: <access_token>` without the word `Bearer` is also accepted.)

Send the **access** token, never the refresh token. Access tokens last **1 hour**, refresh tokens **7 days**.

### CORS
The server only accepts browser requests from `http://localhost:3000`, `:3001`, `:5173` and
`http://127.0.0.1:3000`, `:5173`. A different origin is blocked by the browser (this shows up as a CORS
error, not a 401). Tell the backend team the origin to add.

### Domains
Every user belongs to one domain. **`shutterx` is the root domain**; every other domain is its child.
Responses show the domain by **name** (`"domain": "acme"`); ids of domains are never returned, except a
domain's own `id` in the domain endpoints.

---

## 2. Errors

### 2.1 Status codes

| Code | Meaning |
|---|---|
| 200 / 201 | Success / created |
| 400 | Body is not valid JSON, or an id is malformed |
| 401 | Not signed in – see `reason` below |
| 403 | Signed in but not allowed (wrong role/domain, inactive account or domain) |
| 404 | Not found (also: a permission code the caller's domain cannot see) |
| 409 | Clashes with existing data (duplicate, wrong name/parent for an existing code) |
| 422 | A value is missing or invalid |
| 500 | Server error |

### 2.2 401 – `reason`
The 401 response carries a machine-readable `reason`:

```json
{ "status": false, "code": 401, "reason": "token_expired", "message": "Access token has expired, refresh it or log in again" }
```

| `reason` | What to do |
|---|---|
| `token_expired` | Call `POST /auth/refresh` once, then retry the request. If refresh fails, go to login. |
| `missing_token` | No `Authorization` header was sent. Send it. |
| `bad_authorization_header` | Header is present but not `Bearer <token>`. |
| `wrong_token_type` | The refresh token was sent. Send the access token. |
| `token_invalid` | Token is malformed or signed by a different server secret. Log in again. |
| `token_revoked` | The user logged out. Log in again. |
| `user_not_found` | The account no longer exists. Log in again. |

### 2.3 Validation / conflict errors (`errors` array)
Used by the permission, sharing and role-permission endpoints. `path` points into the request body:

```json
{
  "status": false, "code": 422, "message": "Validation failed, nothing was saved",
  "errors": [
    { "path": "featureGroup.feature[0].permission[1].code", "message": "Code must be 2-32 letters or digits" }
  ]
}
```

All problems are reported at once, and **nothing is saved** when there are any (changes are all-or-nothing).

---

## 3. Auth

### POST /auth/register
Public. Creates an account in a **child** domain (registering into `shutterx` is refused with 403).

```json
{ "domain": "acme", "username": "bob", "name": "Bob", "email": "bob@acme.com", "phone": "9876543210", "password": "min8chars" }
```
`username` and `phone` are optional (username defaults to the part of the email before `@`).
`201` → `data` is the [user object](#user-object).
Errors: 400 payload/validation (domain, name, email, password ≥ 8, username 2–50 chars of `a-z 0-9 . _ -`), 403 domain closed, 404 domain unknown, 409 email or username taken.

### POST /auth/login
Public.

```json
{
  "login": "superadmin@shutterx",
  "password": "secret",
  "device_info": { "device_id": "web-123", "device_type": "web", "os": "Windows", "os_version": "11",
                   "browser": "Chrome", "browser_version": "130", "user_agent": "…" }
}
```
- `login` is `username@domain`. Alternatively send `username` and `domain` as separate fields.
- Existing users may still send their real email (`alice@gmail.com`); it is looked up in the `shutterx` domain.
- `device_info` is optional; `device_id` is needed for device tracking.

`200`:
```json
{
  "status": true, "code": 200, "message": "Login successful",
  "data": {
    "user": { "…": "user object" },
    "tokens": { "access_token": "…", "refresh_token": "…", "token_type": "Bearer", "expires_in": 3600 },
    "device": { "device_id": "web-123", "is_new_device": true },
    "permissions": {
      "featureGroup": ["gRe3"],
      "feature": ["uhMf"],
      "permission": ["vFe3", "Tq8c"]
    }
  }
}
```
`device` is `null` when no `device_info.device_id` was sent.

`permissions` is **only codes**, in three flat lists: the codes of the feature groups, features and permissions the user holds. It is everything granted by **all of the user's roles**, merged (a code granted by two roles appears once). A feature group or feature is listed when the user holds at least one permission under it. All three lists are `[]` when none of the user's roles has any permission. Codes are case-sensitive.

To check access, look for a code in the list: for example `permissions.permission.includes("vFe3")`. Names and the group/feature structure are not returned here; use `GET /permissions` if you need them.

**The `shutterx` superadmin is the exception: it always receives every code in the catalog**, from every domain, whether or not its role has any permission assigned, and codes created later are included automatically.

The lists are a snapshot from login time. `POST /auth/refresh` does not return them, so changes made to the user's roles later only show up the next time they sign in.
Errors: 400 (missing login/password), 401 `Invalid login or password` (unknown domain, user or password – deliberately the same), 403 account or domain inactive.

### POST /auth/refresh
Public. Body: `{ "refresh_token": "…", "device_id": "web-123" }` (`device_id` optional but must match the login device if sent).
`200` → `data: { "tokens": {…}, "device": { "device_id": "web-123" } }`. Replace both stored tokens.
Errors: 401 invalid/expired refresh token or logged-out device, 403 account/domain inactive or token belongs to another device.

### POST /auth/logout
Send the access token header and optionally `{ "refresh_token": "…" }`.
`200` → `data: { "device_id": "web-123", "device_logged_out": true }`. Both tokens stop working.

---

## 4. Shared objects

### User object
```json
{
  "id": 12, "domain": "acme", "username": "bob", "name": "Bob", "email": "bob@acme.com",
  "phone": "9876543210", "status": "active", "last_login": "2026-10-20T09:00:00Z",
  "created_at": "…", "updated_at": "…",
  "roles": [ { "id": 3, "domain": "acme", "name": "manager", "description": null, "created_at": "…" } ]
}
```
`status` is `active`, `inactive` or `blocked`. `roles` is omitted when the user has none.

### Role object
```json
{
  "id": 3, "domain": "acme", "name": "manager", "description": "Runs the studio", "created_at": "…",
  "permissions": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ]
}
```
`permissions` is omitted when the role has none. Only permission-level items are listed, flat.

### Permission tree (the JSON used everywhere for permissions)
```json
{ "featureGroup": {
    "name": "eventManagment", "code": "gRe3",
    "feature": [
      { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" } ] }
    ] } }
```
- Three levels: **featureGroup → feature → permission**; each has a `name` and a `code`.
- `code`: 2–32 letters/digits, **case-sensitive**, unique across the whole catalog. `name`: 1–100 characters.
- When a list is returned, it is an **array of `{ "featureGroup": … }` objects**, in creation order.
  `feature` and `permission` are always arrays (empty `[]` when none).

---

## 5. Users  *(signed-in users; scoped to the caller's domain)*

A normal user only ever sees and changes users of their own domain. The `shutterx` superadmin works in
`shutterx` by default and can add `?domain=<name>` to act in a child domain (list/get also accept `?domain=all`).

| Call | Body / query | Success |
|---|---|---|
| `GET /users?page=1&page_size=10&keyword=bob` | page ≥ 1, page_size 1–100 (default 10), keyword searches name/email/username/phone | `data: { users: [User], total, page, page_size }` |
| `GET /users/:id` | – | `data: User` |
| `POST /users` | `{ "username?", "name", "email", "phone?", "password" (≥8), "status?" ("active"/"inactive"/"blocked"), "role_id?" }` | `201`, `data: User` |
| `PUT /users/:id` | `{ "name?", "phone?", "status?" }` | `data: User` |
| `DELETE /users/:id` | – | `message: "User deleted successfully"` |

Errors: 404 user not found, 409 email/username taken in that domain, 400 `role_id` not in that domain.

---

## 6. Roles  *(signed-in users; scoped to the caller's domain)*

The full roles contract, including create/update with permissions and listing another domain's roles, is in [ROLES_API.md](ROLES_API.md).

| Call | Body | Success |
|---|---|---|
| `GET /roles` | – (`?domain=` for the superadmin) | `data: [Role]` |
| `GET /roles/:id` | – | `data: Role` |
| `POST /roles` | `{ "name", "description?", "codes?", "domain?" }` | `201`, `data: Role` (409 if the name exists in the domain). `codes` gives the role permissions in the same step; `domain` is shutterx superadmin only. |
| `PUT /roles` | `{ "role_id", "name?", "description?", "codes?", "domain?" }` | `data: Role`. Changes only what is sent; `codes` replaces the role's permissions. |
| `POST /roles/list` | `{ "domain?", "keyword?" }` | `data: [Role]` for your own domain or one below it |
| `POST /roles/assign` | `{ "user_id", "role_id" }` | `data: User` (404 unless both are in the domain) |
| `PUT /roles/permissions` | see below | `data: Role` |

### PUT /roles/permissions
Replaces **all** permissions of one role.

```json
{ "role_id": 3, "codes": ["vFe3", "Tq8c"] }
```
- `codes` is required; `[]` removes every permission from the role.
- Only **permission-level** codes the role's domain can see are accepted. For `shutterx` that is every permission;
  for a child domain it is the permissions it created plus what was shared with it (section 8).
- `domain` (optional, `shutterx` superadmin only) acts inside another domain: `{ "role_id": 3, "domain": "acme", "codes": […] }`.
- Anything wrong → `422` with `errors`, e.g. `{ "path": "codes[1]", "message": "Code 'Zz99' is not available to this domain" }`, and nothing changes.

---

## 7. Domains  *(reading: any signed-in user, scoped; creating/editing: shutterx superadmin)*

Domain object:
```json
{ "id": 2, "name": "acme", "display_name": "Acme Studio", "parent": "shutterx", "status": "active", "created_at": "…", "updated_at": "…" }
```
`parent` is `null` for `shutterx` itself.

A studio is created with `POST /studios`, and its saved owner and studio details are read and edited with `GET`/`PUT /studios/:id` (see [STUDIO_ONBOARDING_API.md](STUDIO_ONBOARDING_API.md)). Its parent is the domain of whoever creates it, so the tree can be several levels deep.

| Call | Body | Success |
|---|---|---|
| `GET /domains?page=1&page_size=10&keyword=ac` | – | `data: { domains: [Domain], total, page, page_size }`. Shutterx sees every domain; any other domain sees the domains below it (not itself). |
| `GET /domains/:id` | – | `data: Domain`. Shutterx can read any domain; others only their own or one below it, otherwise 404. |
| `POST /domains` | `{ "name": "acme", "display_name?": "Acme Studio" }` (shutterx superadmin only) | `201`, `data: Domain`, a child of `shutterx` |
| `PUT /domains/:id` | `{ "display_name?", "status?": "active" or "inactive" }` (shutterx superadmin only) | `data: Domain` |

- Domain names: 3–50 chars, lowercase letters, digits and hyphens (no dots), unique ignoring case. 422 if invalid, 409 if taken.
- The `shutterx` domain cannot be modified (403). `POST /domains` always creates a child of `shutterx`; studios created through onboarding are children of the creator's domain.
- An inactive domain blocks login, refresh and all calls for its users.

---

## 8. Permissions

**Anyone signed in can create permissions, whatever their role.** A permission belongs to the domain of the person who created it. `shutterx` sees every permission; a child domain sees the ones it created plus the ones the `shutterx` superadmin has shared with it. **Anyone signed in can also share**: you share the permissions your own domain created with another domain. No role is checked for creating, editing your own domain's permissions, or sharing.

### GET /permissions
What the caller's own domain can see. `shutterx` → every permission from every domain; a child domain → the permissions it
created plus the ones shared with it (with the group/feature names above them; groups with nothing visible are hidden).

```json
{ "status": true, "code": 200, "message": "Permissions retrieved successfully",
  "data": [
    { "featureGroup": { "name": "eventManagment", "code": "gRe3",
        "feature": [ { "name": "eventBoard", "code": "uhMf",
          "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } },
    { "featureGroup": { "name": "clientManagment", "code": "k9Qx", "feature": [] } }
  ] }
```

### POST /permissions/view
Read with options. Body is optional; both fields are optional.

```json
{ "code": "Tq8c" }
```
```json
{ "domain": "acme" }
```
- `code` → `data` is **one** `{ "featureGroup": … }`: for a group code the whole group, for a feature code that feature with its permissions, for a permission code just that one. `404 "Permission not found"` if the code does not exist **or is not visible to the domain**.
- `domain` (`shutterx` superadmin only) → what that domain sees. Another domain's user sending a different `domain` gets 403.
- `owned: true` → only the permissions your own domain created. Use it to draw the share screen, so you only offer what you can share.
- `shared_with: "acme"` → only your own permissions that you already share with acme. Use it to pre-tick the share screen. Cannot be combined with `owned`.
- None of these → same as `GET /permissions`.

### POST /permissions   *(any signed-in user)*
Creates permissions owned by the caller's domain. Send your tree, or an array of trees. **Every level needs a `name` and a `code`.**

```json
{ "featureGroup": { "name": "eventManagment", "code": "gRe3",
    "feature": [ { "name": "eventBoard", "code": "uhMf",
      "permission": [ { "name": "read", "code": "vFe3" } ] } ] } }
```
- Send a group alone: `{ "featureGroup": { "name": "clientManagment", "code": "k9Qx" } }`.
- To add a feature/permission under something your domain created, send its parents with their existing codes and put the new item inside.
- Items are matched by `code`: an existing, identical item of your domain is skipped (not duplicated).
- A code owned by another domain, or a group owned by another domain (even one shared with you), cannot be used or extended: `409` `Code '…' is already in use`.
- `201` with `data: { "created": 3, "existing": 0, "featureGroup": {…} }` (array input → `featureGroups: [ {featureGroup}, … ]`); `200` with `created: 0` when everything already existed.
- `409` when a code already exists as another level / under another parent / with another name, or a name is already used under the same parent. `422` for missing/invalid names or codes, unknown JSON keys (e.g. `features`), or duplicate codes inside the request.
- Creating permissions only saves them to the permission table. No role is given anything automatically; use `PUT /roles/permissions` to give a role its permissions.

### PUT /permissions   *(any signed-in user, own domain's permissions only)*
Renames an item and/or changes its code. The item must have been created by the caller's domain, otherwise `403`.

```json
{ "code": "Tq7b", "name": "edit", "new_code": "Tq8c" }
```
`code` = the item to change; send `name` and/or `new_code`. `200` → `data` is the updated group branch
(`{ "featureGroup": … }`, only that item inside). `404` unknown code, `403` the item belongs to another domain, `409` new code taken or name already used at that level in your domain, `422` invalid values.
Changing a code does not affect roles or sharing; it **will** break any frontend code that hard-codes the old code.

### PUT /permissions/share   *(any signed-in user)*
Chooses which of **your domain's own** permissions one other domain can see – many permissions in one request.

```json
{
  "domain": "acme",
  "mode": "replace",
  "dry_run": false,
  "permissions": [
    { "featureGroup": { "name": "eventManagment", "code": "gRe3",
      "feature": [ { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } }
  ]
}
```

| Field | Notes |
|---|---|
| `domain` | Required. The domain to share with. Your own domain and `shutterx` are rejected (422) – shutterx already sees everything. |
| `permissions` | A subset of the tree from `POST /permissions/view` with `{ "owned": true }` (one object or an array). Not needed for `clear`. |
| `mode` | `replace` (default), `add`, `remove`, `clear` |
| `dry_run` | `true` = report what would change, change nothing |

Modes:

| `mode` | Effect |
|---|---|
| `replace` | The request becomes the full set **you** share with that domain. Anything you had shared that is not in the request is removed. Use this for a share screen with ticked boxes. |
| `add` | Adds the permissions in the request; everything else stays. |
| `remove` | Removes the permissions in the request; everything else stays. |
| `clear` | Removes everything **you** share with that domain. |

Rules:
- You can only share permissions your own domain created. A code from another domain, or one that was shared with you, is reported as `Code '…' does not exist in your domain's permissions`. You cannot pass on what you received.
- `replace`, `remove` and `clear` only touch what you shared. Permissions the target received from other domains are never affected.
- Every `name`/`code` must match your permissions exactly (and sit under the right parent); otherwise `422`/`409` with `errors` and nothing changes.
- Only **permission-level** entries grant access. A group or feature sent with no permissions inside shares nothing. A new permission added to a group later is **not** shared automatically.
- Permissions that are removed from a domain are also removed from that domain's roles, in the same step.

`200`:
```json
{ "status": true, "code": 200, "message": "Permissions shared successfully",
  "data": {
    "domain": "acme", "mode": "replace", "dry_run": false,
    "added": 2, "removed": 0, "roles_affected": 0,
    "permissions": [ { "featureGroup": { "…": "what the domain can now see" } } ]
  } }
```
`permissions` is what **you** now share with that domain, not everything the domain can see. `roles_affected` = number of that domain's roles that lost at least one permission. With `dry_run: true` the message is `Dry run, nothing was changed` and the counts show what would happen.

---

## 9. Frontend flows

### Sign-in and every request
1. `POST /auth/login` → store `tokens.access_token` and `tokens.refresh_token`.
2. Send `Authorization: Bearer <access_token>` on every other call.
3. On `401` with `reason: "token_expired"` → `POST /auth/refresh`, store the new pair, retry once. Any other 401 `reason` → login screen.

### Share screen (any domain)
1. `POST /permissions/view` with `{ "owned": true }` – the permissions your domain can share; render the checkbox tree.
2. `POST /permissions/view` with `{ "shared_with": "acme" }` – what you already share with acme; pre-tick those codes.
3. User ticks/unticks. Build the tree containing **only ticked permissions** (keep each ticked permission's feature and group, with their names and codes).
4. Optional preview: `PUT /permissions/share` with `"dry_run": true` – show `removed` / `roles_affected` in a confirm dialog.
5. Save: `PUT /permissions/share` with `"mode": "replace"`.

### What a child domain sees
`GET /permissions` – only the shared tree. Use it to render the role editor, then `PUT /roles/permissions` with the chosen `codes`.

---

## 10. Health

`GET /` and `GET /health` return `{ "status": … }` without authentication.
