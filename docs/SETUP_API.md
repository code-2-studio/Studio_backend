# First-run Setup API

How to start a **brand-new system**: create the first superadmin of the `shutterx` domain, and optionally load the
first permission catalog, with one API call.

Base URL: `http://<host>:<port>/live/api/v1`

> Written from the code. The validation, the key check and the rate limiter are unit-tested; the HTTP endpoints and
> the database transaction have not been run against a live database yet.

---

## 1. What a new system needs

On every start the server creates the database tables and the `shutterx` root domain by itself. A system is usable once
three more things exist, and this API creates them:

| Table | First row | Created by |
|---|---|---|
| `domains` | `shutterx` (display name `ShutterX`, no parent, `active`) | **the server at startup** |
| `roles` | `superadmin` in shutterx (created empty) | **setup** |
| `users` | the superadmin user, in shutterx | **setup** |
| `user_roles` | links the user to the role | **setup** |
| `permissions` | optional first catalog, owned by shutterx | **setup** (optional) |
| `role_permissions`, `domain_permissions`, `domain_profiles`, `user_devices` | nothing | filled later |

The superadmin role does not need any permission rows: the shutterx superadmin receives **every** permission at login.

## 2. Two ways to create the first superadmin

| Way | When to use |
|---|---|
| **Environment variable seed** (existing) | Automated deploys. Set `SUPERADMIN_PASSWORD` (8+ characters, optionally `SUPERADMIN_USERNAME`, `SUPERADMIN_EMAIL`) and the server creates `superadmin@shutterx` on startup. |
| **Setup API** (this document) | Manual first run, with your own name, username and email, and optionally the first permission catalog. |

If the env seed has already created the superadmin, the setup API answers `409 System is already initialized`.

## 3. Turn it on: the setup key

The setup call is protected by a secret, because it creates the most powerful account in the system and is reachable
before anyone has logged in.

1. Choose a long random value and set it as the environment variable **`SETUP_KEY`** on the server.
   ```env
   SETUP_KEY=put-a-long-random-secret-here
   ```
2. Restart the server. **If `SETUP_KEY` is not set, `POST /setup` is switched off** and answers `404 Setup is not enabled`.
3. After setup succeeds, remove the variable (or change it). It is no longer needed.

The key is only ever sent in the request body, compared in constant time, and never logged.

---

## 4. GET /setup/status
Public. No body. Use it to decide whether to show the setup screen.

```json
{ "status": true, "code": 200, "message": "Setup status",
  "data": { "initialized": false, "setup_enabled": true } }
```

| Field | Meaning |
|---|---|
| `initialized` | `true` once a superadmin exists in shutterx. Show the normal login screen. |
| `setup_enabled` | `true` when the server has a `SETUP_KEY`. If `initialized` is `false` and this is `false`, setup cannot run until the server is configured. |

## 5. POST /setup
Public, but needs the setup key. Works **once**. Rate limited to 5 requests per minute per IP.

### Request
```json
{
  "setup_key": "put-a-long-random-secret-here",
  "superadmin": {
    "full_name": "Asha Rao",
    "username": "superadmin",
    "email": "superadmin@shutterx",
    "mobile": "9876543210",
    "password": "a-strong-password"
  },
  "permissions": [
    { "featureGroup": { "name": "eventManagment", "code": "gRe3",
      "feature": [ { "name": "eventBoard", "code": "uhMf",
        "permission": [ { "name": "read", "code": "vFe3" }, { "name": "edit", "code": "Tq8c" } ] } ] } }
  ]
}
```

| Field | Required | Rules |
|---|---|---|
| `setup_key` | yes | must equal the server's `SETUP_KEY` |
| `superadmin.full_name` | yes | up to 100 characters |
| `superadmin.username` | no | 2–50 characters: `a-z 0-9 . _ -`. **Defaults to `superadmin`.** |
| `superadmin.email` | no | a valid email, stored lower-case. **Defaults to `<username>@shutterx`.** |
| `superadmin.mobile` | no | 10–15 digits, optional leading `+` |
| `superadmin.password` | yes | 8–72 characters |
| `permissions` | no | the first catalog: one `{ "featureGroup": … }` or an array, exactly as in `POST /permissions` (every level needs a `name` and a `code`). Owned by shutterx. |

The shortest valid request:
```json
{ "setup_key": "…", "superadmin": { "full_name": "Asha Rao", "password": "a-strong-password" } }
```
This creates the user `superadmin@shutterx`.

### What it does
In **one transaction**, after every check has passed (so a rejected request leaves nothing behind):
1. Makes sure the `shutterx` domain exists.
2. Creates the `superadmin` role in shutterx, if it is not there already.
3. Creates the superadmin user and gives them the role.
4. Saves the optional permission catalog.

Two requests at the same moment cannot both succeed: a database lock lets one through and the other gets `409`.

### 201 Created
```json
{
  "status": true, "code": 201, "message": "System initialized",
  "data": {
    "login": "superadmin@shutterx",
    "user": {
      "id": 1, "domain": "shutterx", "username": "superadmin", "name": "Asha Rao",
      "email": "superadmin@shutterx", "phone": "+919876543210", "status": "active", "last_login": null,
      "created_at": "…", "updated_at": "…",
      "roles": [ { "id": 1, "domain": "shutterx", "name": "superadmin",
                   "description": "Full access to the root domain and every child domain", "created_at": "…" } ]
    },
    "permissions_created": 4
  }
}
```
`permissions_created` counts every group, feature and permission created (here 1 group, 1 feature, 2 permissions).
No tokens are returned. Sign in with the normal login:
```json
{ "login": "superadmin@shutterx", "password": "a-strong-password" }
```
The login response then lists every permission code in `data.permissions`.

### Errors
Nothing is created when there is any error.

| Code | When |
|---|---|
| `404` | `Setup is not enabled`: the server has no `SETUP_KEY` |
| `403` | `Invalid setup key` |
| `409` | `System is already initialized` (a superadmin already exists) |
| `409` | the username or email is already used in shutterx, or a permission code clashes (see below) |
| `422` | a value is missing or invalid |
| `429` | more than 5 attempts in a minute from the same IP; wait and retry |
| `400` | the body is not valid JSON |

`422` example (every problem is listed, with its JSON path):
```json
{ "status": false, "code": 422, "message": "Validation failed, nothing was created",
  "errors": [
    { "path": "superadmin.full_name", "message": "Full name is required" },
    { "path": "superadmin.password", "message": "Password must be at least 8 characters long" },
    { "path": "permissions[0].featureGroup.feature[0].code", "message": "Code is required" }
  ] }
```
`409` example:
```json
{ "status": false, "code": 409, "message": "Setup failed, nothing was created",
  "errors": [ { "path": "superadmin.username", "message": "Username is already used in the shutterx domain" } ] }
```
That one happens on a database that already had users (they are moved into shutterx): choose another username.

---

## 6. Step by step for a new system
1. Deploy the server with the database settings and a `JWT_SECRET_KEY`. Add `SETUP_KEY`.
2. Start it. The tables and the `shutterx` domain are created automatically.
3. `GET /setup/status`: `initialized` is `false`, `setup_enabled` is `true`.
4. `POST /setup` with the payload above.
5. `POST /auth/login` with `superadmin@shutterx` and the password.
6. Remove `SETUP_KEY` from the server's environment.
7. From here on the superadmin creates studios (`POST /studios`), roles and permissions, as in the other documents.

## 7. Good to know
- **Lost password:** setup runs only once. There is no API to reset the first superadmin's password; that needs a change in the database or a separate recovery feature.
- **The setup key is a secret.** Keep it out of source control. (Note that `.env` is tracked in this repository, so put `SETUP_KEY` in the server's real environment, not in that file.)
- **The rate limit** is kept in the server's memory: it resets on restart and is per server instance.
