# Studio Onboarding API Contract

Base URL: `http://<host>:<port>/live/api/v1`

All parameters go in the JSON body. The exceptions are `GET`/`PUT /studios/:id` and `GET /domains`, which keep the id in the path or `?` as before.

> Written from the code. The validation and response building are unit-tested; the HTTP endpoints and the database
> transaction have not been run against a live database yet. The studio **logo is not included** yet.

## What onboarding does

One call creates a whole studio, in a single transaction (all or nothing):

1. A new **domain** (the unique studio handle, e.g. `mystudio`). Its **parent is the domain of the signed-in user who
   creates it**, so a shutterx user creates a child of `shutterx`, and a user of `acme` creates a child of `acme`.
2. The studio and owner **details** (stored with the domain).
3. A role named **Admin** in the new domain. It starts with **no permissions** (nothing is granted to a role automatically).
4. An **admin user** in the new domain with the Admin role, using the username and password you send.
   They sign in as `username@domain`, e.g. `admin@mystudio`.

**Headers (every call)**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```
Any signed-in user can onboard a studio, whatever their role.

**Response envelope**
```json
{ "status": true, "code": 201, "message": "…", "data": … }
```

---

## 1. POST /studios/check-domain
Use while the user types the domain. Body: `{ "domain": "mystudio" }`

`200` available:
```json
{ "status": true, "code": 200, "message": "Domain checked",
  "data": { "domain": "mystudio", "available": true, "admin_login": "admin@mystudio" } }
```
`200` not available (`reason` is `taken` or `reserved`):
```json
{ "status": true, "code": 200, "message": "Domain checked",
  "data": { "domain": "acme", "available": false, "reason": "taken", "admin_login": "admin@acme" } }
```
`422` when the format is wrong:
```json
{ "status": false, "code": 422, "message": "Validation failed",
  "errors": [ { "path": "domain", "message": "Domain must be 3-50 characters: lowercase letters, digits and hyphens only" } ] }
```
The domain is lower-cased for you (`MyStudio` → `mystudio`). Names such as `admin`, `api`, `www`, `app`, `mail`, `root`, `support`, `help`, `login`, `register`, `dashboard`, `superadmin`, `studio` are reserved.
A name can still be taken between this check and the create call; the create call re-checks.

## 2. POST /studios
Creates the studio.

```json
{
  "owner": {
    "full_name": "Asha Rao",
    "mobile": "+91 98765 43210",
    "business_email": "asha@mystudio.com"
  },
  "studio": {
    "name": "My Studio",
    "domain": "mystudio",
    "address": "12 MG Road",
    "city": "Pune",
    "state": "Maharashtra",
    "pincode": "411001",
    "whatsapp": "9876543210",
    "gstin": "27ABCDE1234F1Z5",
    "website": "https://mystudio.com",
    "instagram": "@mystudio",
    "facebook": "https://facebook.com/mystudio",
    "youtube": "@mystudio"
  },
  "admin": { "username": "admin", "password": "longenough" }
}
```

### Fields

| Field | Required | Rules |
|---|---|---|
| `owner.full_name` | yes | up to 100 characters |
| `owner.mobile` | yes | 10–15 digits, optional leading `+`. Spaces, `-`, `.` and brackets are ignored. |
| `owner.business_email` | yes | a valid email. Stored lower-case, and becomes the admin user's email. |
| `studio.name` | yes | up to 150 characters. Shown as the domain's `display_name`. |
| `studio.domain` | yes | 3–50 lowercase letters, digits and hyphens (no dots), unique, not reserved |
| `studio.address` | yes | up to 255 characters |
| `studio.city` | yes | up to 100 characters |
| `studio.state` | yes | up to 100 characters |
| `studio.pincode` | yes | exactly 6 digits |
| `studio.whatsapp` | no | same rule as mobile |
| `studio.gstin` | no | a valid 15-character GSTIN. Upper-cased for you. |
| `studio.website` | no | a URL. A missing `https://` is added. A blank value or just `https://` counts as not given. |
| `studio.instagram` | no | an `@handle` (an `@` is added if missing) or a profile URL |
| `studio.facebook` | no | an `@handle` or a profile URL |
| `studio.youtube` | no | an `@handle` or a channel URL |
| `admin.username` | no | 2–50 characters: `a-z 0-9 . _ -`. **Defaults to `admin`** when left blank. |
| `admin.password` | yes | 8–72 characters |

Unknown keys (for example a `logo` field) are ignored.

### 201 Created
```json
{
  "status": true, "code": 201, "message": "Studio created successfully",
  "data": {
    "domain": {
      "id": 7, "name": "mystudio", "display_name": "My Studio", "parent": "shutterx",
      "status": "active", "created_at": "2026-10-20T10:00:00Z", "updated_at": "2026-10-20T10:00:00Z"
    },
    "owner": { "full_name": "Asha Rao", "mobile": "+919876543210", "business_email": "asha@mystudio.com" },
    "studio": {
      "name": "My Studio", "domain": "mystudio", "address": "12 MG Road", "city": "Pune",
      "state": "Maharashtra", "pincode": "411001", "whatsapp": "9876543210", "gstin": "27ABCDE1234F1Z5",
      "website": "https://mystudio.com", "instagram": "@mystudio",
      "facebook": "https://facebook.com/mystudio", "youtube": "@mystudio"
    },
    "admin": {
      "login": "admin@mystudio",
      "username": "admin",
      "role": "Admin",
      "user": {
        "id": 21, "domain": "mystudio", "username": "admin", "name": "Asha Rao",
        "email": "asha@mystudio.com", "phone": "+919876543210", "status": "active", "last_login": null,
        "created_at": "…", "updated_at": "…",
        "roles": [ { "id": 9, "domain": "mystudio", "name": "Admin", "description": "Studio administrator", "created_at": "…" } ]
      }
    }
  }
}
```
Optional values that were not sent come back as `null`. `domain.parent` is the name of the domain that created the studio.
No tokens are returned; the admin signs in with `POST /auth/login`:
```json
{ "login": "admin@mystudio", "password": "longenough" }
```

### Errors
All problems are reported at once, and **nothing is created** when there are any.

`422` (missing or invalid values):
```json
{ "status": false, "code": 422, "message": "Validation failed, nothing was created",
  "errors": [
    { "path": "owner.business_email", "message": "Business email is not a valid email address" },
    { "path": "studio.pincode", "message": "Pincode must be 6 digits" },
    { "path": "admin.password", "message": "Password must be at least 8 characters long" }
  ] }
```
`path` is the JSON path of the field, so you can show each message next to its input.

`409` (domain already taken):
```json
{ "status": false, "code": 409, "message": "Conflicts found, nothing was created",
  "errors": [ { "path": "studio.domain", "message": "Domain name is already taken" } ] }
```

`400` the body is not valid JSON. `401` not signed in (the `reason` field says why, e.g. `token_expired`).

---

## 3. Reading and editing a studio

`:id` is the studio's **domain id**, the same `id` you get from `GET /domains` and from the onboarding response (`data.domain.id`).

### GET /studios/:id
Returns the saved owner and studio details, in the same `owner` and `studio` objects that onboarding accepts, so an edit form can be filled from it.

Who can read: the studio's own users, the domains above it (any depth), and shutterx. Anyone else gets `404 Studio not found`.

`200`
```json
{
  "status": true, "code": 200, "message": "Studio retrieved successfully",
  "data": {
    "domain": {
      "id": 7, "name": "mystudio", "display_name": "My Studio", "parent": "shutterx",
      "status": "active", "created_at": "…", "updated_at": "…"
    },
    "owner": { "full_name": "Asha Rao", "mobile": "+919876543210", "business_email": "asha@mystudio.com" },
    "studio": {
      "name": "My Studio", "domain": "mystudio", "address": "12 MG Road", "city": "Pune",
      "state": "Maharashtra", "pincode": "411001", "whatsapp": "9876543210", "gstin": "27ABCDE1234F1Z5",
      "website": "https://mystudio.com", "instagram": "@mystudio",
      "facebook": "https://facebook.com/mystudio", "youtube": "@mystudio"
    }
  }
}
```
Optional values that were never given are `null`.

A domain that was not created through onboarding (for example `shutterx`, or one made with `POST /domains`) has no saved details. For those, `owner` and `studio` are both `null` and only `domain` is filled. An edit form should start empty for them; saving it creates the details.

### PUT /studios/:id
Replaces the owner and studio details. The body is the onboarding body **without** the `admin` section, and the domain is locked:

```json
{
  "owner": { "full_name": "Asha Rao", "mobile": "+91 98765 43210", "business_email": "asha@mystudio.com" },
  "studio": {
    "name": "My Studio", "address": "12 MG Road", "city": "Pune", "state": "Maharashtra", "pincode": "411001",
    "whatsapp": "9876543210", "gstin": "27ABCDE1234F1Z5", "website": "https://mystudio.com",
    "instagram": "@mystudio", "facebook": "https://facebook.com/mystudio", "youtube": "@mystudio"
  }
}
```
- **Same field rules and error format as onboarding** (see the field table in section 2). The required fields must always be sent.
- **Send the whole form.** This replaces the details; it is not a partial update. An optional field that is blank or left out is cleared (it comes back `null`).
- `studio.domain` can be left out. If you send it, it must be the current domain (any case); a different one is rejected with `studio.domain: The domain cannot be changed`. This lets you reuse the onboarding form with the domain field locked.
- `studio.name` also becomes the domain's `display_name`.
- An `admin` section, if sent, is ignored. The admin user is not changed, **and neither is the admin's email when the business email changes**. Changing the admin password is not available yet.

Who can edit: the studio's **own users**, and the **shutterx superadmin**. No role is checked for the studio's own users. A parent domain can read a studio below it but not edit it (`403`).

`200`: the same body as `GET /studios/:id`, with the new values, and `"message": "Studio updated successfully"`.

Errors:

`422`
```json
{ "status": false, "code": 422, "message": "Validation failed, nothing was changed",
  "errors": [
    { "path": "studio.pincode", "message": "Pincode must be 6 digits" },
    { "path": "studio.domain", "message": "The domain cannot be changed" }
  ] }
```
`400` invalid JSON or a non-numeric id. `401` not signed in. `404 Studio not found` (no such studio, or you cannot see it).
`403` with one of: `Forbidden: Only the studio's own users or the shutterx superadmin can edit it`, or `The root domain cannot be modified` (the `shutterx` domain itself cannot be edited).

---

## 4. Seeing the studios you created

### GET /domains?page=1&page_size=10&keyword=my
Any signed-in user. You see the domains **below yours**, at any depth (the studios you onboarded and the ones they onboarded). The shutterx domain sees every domain. Your own domain is not in the list.

`200`
```json
{ "status": true, "code": 200, "message": "Domains retrieved successfully",
  "data": { "domains": [
      { "id": 7, "name": "mystudio", "display_name": "My Studio", "parent": "shutterx", "status": "active", "created_at": "…", "updated_at": "…" }
    ], "total": 1, "page": 1, "page_size": 10 } }
```
`page_size` is 1–100 (default 10). `keyword` matches `name` and `display_name`, ignoring case.

### GET /domains/:id
`200` with one domain object (the same fields as above). You can read your own domain or one below it; any other id returns `404 Domain not found`.

Creating a bare domain (`POST /domains`) and changing a domain's status (`PUT /domains/:id`) stay limited to the shutterx superadmin. A parent domain does not manage its studios' users or roles; it only has the link and the list.

---

## Notes
- **Admin role permissions:** the Admin role starts empty. Give it permissions with `PUT /roles/permissions` (see the permissions contract). A new studio sees only the permissions it created or was shared.
- **Sign-up still open:** `POST /auth/register` lets anyone self-register into any non-shutterx domain, including a new studio. Say if you want that closed or limited.
- **Not included yet:** studio logo upload, changing the admin's password, deleting a studio.
