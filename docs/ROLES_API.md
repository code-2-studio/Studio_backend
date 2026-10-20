# Roles API Contract

Base URL: `http://<host>:<port>/live/api/v1`

Role create, update and list take their parameters in the **JSON body**.

> Written from the code. The validation rules are unit-tested; the HTTP endpoints and the database transactions
> have not been run against a live database yet.

**Headers (every call)**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Response envelope**
```json
{ "status": true, "code": 200, "message": "…", "data": … }
```

**Who can call:** any signed-in user, whatever their role. By default a call works on **your own domain's** roles.
Naming another domain in the body is covered under each call.

## The role object
```json
{
  "id": 9, "domain": "mystudio", "name": "Photographer", "description": "Shoots events", "created_at": "2026-10-20T10:00:00Z",
  "permissions": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ]
}
```
`description` is `null` when empty. `permissions` is left out when the role has none. Roles are listed with the permissions
as a flat `[{ name, code }]`; the permission codes are the ones from the permissions API.

**Role permissions:** a role can only be given permission codes its domain can see. For `shutterx` that is every permission;
for any other domain it is the permissions the domain created plus the ones shared with it.

---

## 1. POST /roles/list
Lists the roles of one domain, each with its permissions. Body (everything optional):

```json
{ "domain": "mystudio", "keyword": "photo" }
```

| Field | Meaning |
|---|---|
| `domain` | Whose roles to list. Leave out for your own domain. You may name a domain **below yours**, at any depth (the studios you onboarded and theirs). `shutterx` may name any domain. |
| `keyword` | Matches part of the role name, ignoring case |

`200`
```json
{ "status": true, "code": 200, "message": "Roles retrieved successfully",
  "data": [
    { "id": 9, "domain": "mystudio", "name": "Admin", "description": "Studio administrator", "created_at": "…" },
    { "id": 12, "domain": "mystudio", "name": "Photographer", "description": null, "created_at": "…",
      "permissions": [ { "name": "read", "code": "vFe3" } ] }
  ] }
```
Roles are in creation order. A domain you are not allowed to see returns `404 Domain not found`.
`GET /roles` still returns your own domain's roles, without a body.

## 2. POST /roles
Creates a role, and can give it its permissions in the same step.

```json
{
  "name": "Photographer",
  "description": "Shoots events",
  "codes": ["vFe3", "Tq8c"]
}
```

| Field | Required | Rules |
|---|---|---|
| `name` | yes | up to 100 characters, unique within the domain |
| `description` | no | up to 255 characters |
| `codes` | no | permission codes to give the role. Leave out for a role with no permissions. |
| `domain` | no | create the role inside another domain. **Only the shutterx superadmin can do this**; anyone else gets `403`. |

`201`: `data` is the new role (with its permissions), message `Role created successfully`.

Errors (nothing is created when there is any):

`422` a value is wrong, or a code is not available to the role's domain:
```json
{ "status": false, "code": 422, "message": "Some codes are not available, nothing was saved",
  "errors": [ { "path": "codes[1]", "message": "Code 'Zz99' is not available to this domain" } ] }
```
`409` the name is already used in that domain:
```json
{ "status": false, "code": 409, "message": "Conflicts found, nothing was saved",
  "errors": [ { "path": "name", "message": "Role name already exists in this domain" } ] }
```
In the `shutterx` domain the name `superadmin` is reserved (`422`, `This role name is reserved`), because that role grants
cross-domain powers. In any other domain it is an ordinary name.

## 3. PUT /roles
Changes a role's name, description and/or permissions. Send `role_id` and **only what should change**.

```json
{
  "role_id": 12,
  "name": "Lead Photographer",
  "description": "Leads the shoots",
  "codes": ["vFe3"]
}
```

| Field | Meaning |
|---|---|
| `role_id` | Required. The role to change. |
| `name` | New name. Same rules as create. Sending the current name changes nothing. |
| `description` | New description. `""` clears it. |
| `codes` | **Replaces all** of the role's permissions with these. `[]` removes them all. Leave the field out to keep the permissions as they are. |
| `domain` | Change a role inside another domain. Shutterx superadmin only (`403` for anyone else). |

At least one of `name`, `description` or `codes` is needed. Everything is saved together or not at all.

`200`: `data` is the updated role, message `Role updated successfully`.

Errors:
- `422` nothing to change, an invalid value, or a code that is not available (`errors` lists each one by path, e.g. `codes[0]`).
- `409` the new name is used by another role in that domain.
- `403` `The superadmin role cannot be renamed` (the `superadmin` role of `shutterx`; its permissions can still be changed), or a `domain` you may not use.
- `404` `Role not found in this domain`.

## 4. PUT /roles/permissions
Replaces only a role's permissions. Same effect as `codes` in `PUT /roles`.
```json
{ "role_id": 12, "codes": ["vFe3", "Tq8c"] }
```
`200` with the role. See the permissions contract for the rules.

---

## Other role calls (unchanged)

| Call | Body | Notes |
|---|---|---|
| `GET /roles` | – | Your own domain's roles |
| `GET /roles/:id` | – | One role of your domain |
| `POST /roles/assign` | `{ "user_id", "role_id" }` | Gives a user a role of the same domain |

## Flow for a role screen
1. `POST /roles/list` with `{ "domain": "mystudio" }` to show that domain's roles.
2. `GET /permissions` to show the permissions the domain can choose from (or `POST /permissions/view` for another domain you may see).
3. Create: `POST /roles` with `name` and `codes`. Edit: `PUT /roles` with `role_id` and the changed fields.

## Not built
Deleting a role. Removing a role from a user.
