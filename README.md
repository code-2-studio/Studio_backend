# Studio Backend CRM

**A Studio Management CRM for Photography and Videography Businesses**

Studio Backend CRM is a backend application built to manage the daily operations of photography and video production studios. It provides APIs and administrative functionality for managing clients, bookings, studio projects, staff, services, payments, and reports.

The application uses **Go with the Fiber web framework** and **PostgreSQL** for database management. It is designed to support role-based access control (RBAC), secure API access, and scalable studio operations.

## Table of Contents

- [Project Overview](#project-overview)
- [Technology Stack](#technology-stack)
- [Core Modules](#core-modules)
- [Role-Based Access Control](#role-based-access-control)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Installation and Setup](#installation-and-setup)
- [Environment Configuration](#environment-configuration)
- [Database Setup](#database-setup)
- [Running the Application](#running-the-application)
- [API Structure](#api-structure)
- [Security](#security)
- [Testing](#testing)
- [Deployment](#deployment)
- [Git Workflow](#git-workflow)

## Project Overview

The Studio Backend CRM centralizes customer management and studio workflows in one system.

The backend is responsible for:

- Managing clients and their contact information.
- Scheduling photography and videography sessions.
- Tracking projects, assignments, and delivery status.
- Managing studio services and packages.
- Recording payments, invoices, and outstanding balances.
- Managing employees, user accounts, roles, and permissions.
- Providing administrative APIs and operational reports.

## Technology Stack

| Technology | Purpose |
|---|---|
| Go (Golang) | Backend programming language |
| Fiber | HTTP web framework and REST APIs |
| PostgreSQL | Relational database |
| SQL / PostgreSQL queries | Data access |
| Go Modules | Dependency management |
| Git and GitHub | Version control |
| Environment variables | Application configuration |

Authentication, database migration, logging, and API documentation libraries should be documented here once the specific packages used by the project are finalized.

## Core Modules

### 1. Admin Dashboard

- Provide an overview of studio operations.
- Display booking and project statistics.
- Track pending payments and upcoming sessions.
- Show operational summaries and recent activities.

### 2. Client Management

- Create and manage client profiles.
- Store contact information and customer details.
- View booking and project history.
- Track client-related transactions.

### 3. Booking Management

- Create and update studio bookings.
- Schedule photoshoots and video shoots.
- Track booking confirmation, completion, and cancellation.
- Manage session dates, locations, and assigned staff.

### 4. Project Management

- Create projects linked to clients and bookings.
- Assign photographers, videographers, and other staff.
- Track editing, review, and delivery stages.
- Manage project deadlines and status updates.

### 5. Service and Package Management

- Manage photography and videography services.
- Configure packages and pricing.
- Maintain service descriptions and availability.
- Associate services with client bookings.

### 6. Payment and Invoice Management

- Record advance payments and transactions.
- Track pending balances.
- Maintain invoice and payment records.
- Generate financial summaries and reports.

### 7. User Management

- Create and manage staff accounts.
- Assign roles to users.
- Activate or deactivate accounts.
- Manage access to administrative functions.

### 8. Reports and Analytics

- Booking reports.
- Project status reports.
- Client activity reports.
- Payment and revenue summaries.
- Staff and operational reports.

## Role-Based Access Control

The application is designed to implement Role-Based Access Control (RBAC), ensuring that users can perform only the operations authorized for their assigned roles.

| Role | Responsibilities |
|---|---|
| Super Admin | Full system administration and configuration |
| Studio Owner | Studio operations, clients, bookings, and business reports |
| Manager | Booking coordination, project management, and staff assignments |
| Staff | Access to assigned projects and permitted tasks |
| Accountant | Payments, invoices, and financial reports |

The exact permissions should be configurable based on the studio's requirements.

### Recommended RBAC Database Tables

- `users` — user accounts and authentication details.
- `roles` — available system roles.
- `permissions` — individual access permissions.
- `user_roles` — maps users to roles.
- `role_permissions` — maps roles to permissions.

The backend must validate permissions on every protected operation. Hiding menu items in an admin panel alone is not sufficient authorization.

## Project Structure

The following is a recommended structure for the Go Fiber backend. Adapt it to the folders already present in the repository.

```text
studio-backend/
├── main.go
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
├── README.md
│
├── config/
│   ├── config.go
│   └── database.go
│
├── routes/
│   └── routes.go
│
├── controllers/
│   ├── auth_controller.go
│   ├── user_controller.go
│   ├── client_controller.go
│   ├── booking_controller.go
│   ├── project_controller.go
│   └── payment_controller.go
│
├── middleware/
│   ├── auth.go
│   ├── permission.go
│   └── error_handler.go
│
├── models/
│   ├── user.go
│   ├── role.go
│   ├── permission.go
│   ├── client.go
│   ├── booking.go
│   └── project.go
│
├── repositories/
│   ├── user_repository.go
│   ├── client_repository.go
│   ├── booking_repository.go
│   └── project_repository.go
│
├── services/
│   ├── auth_service.go
│   ├── client_service.go
│   └── booking_service.go
│
├── database/
│   └── migrations/
│
├── utils/
│   ├── response.go
│   └── validation.go
│
└── tests/
```

### Architecture Responsibilities

- **Config:** Loads environment variables and initializes application configuration.
- **Routes:** Registers API endpoints and route groups.
- **Controllers:** Handles HTTP requests and responses.
- **Services:** Contains business rules and application workflows.
- **Repositories:** Handles database operations.
- **Models:** Defines application and database data structures.
- **Middleware:** Handles authentication, authorization, and request processing.
- **Database migrations:** Maintains version-controlled database schema changes.
- **Utils:** Contains reusable helpers and response utilities.

## Prerequisites

Install the following tools before running the application:

- Go, using a version compatible with `go.mod`.
- PostgreSQL.
- Git.
- A terminal or code editor.

Verify your installation:

```bash
go version
git --version
psql --version
```

## Installation and Setup

### 1. Clone the Repository

```bash
git clone <YOUR_REPOSITORY_URL>
cd studio-backend
```

Replace `<YOUR_REPOSITORY_URL>` with the actual Git repository URL.

### 2. Download Dependencies

```bash
go mod download
```

To verify the dependency configuration:

```bash
go mod tidy
```

Run `go mod tidy` when intentionally updating dependencies; review the changes to `go.mod` and `go.sum` before committing them.

### 3. Configure Environment Variables

Create a local `.env` file from the example configuration:

```bash
cp .env.example .env
```

Update the values to match your local environment.

### 4. Create the PostgreSQL Database

Open PostgreSQL:

```bash
psql -U postgres
```

Create the application database:

```sql
CREATE DATABASE studio_crm;
```

Exit the PostgreSQL shell:

```sql
\q
```

Configure the application with a PostgreSQL user that has the necessary database permissions.

### 5. Run Database Migrations

Apply the database migrations using the migration tool configured by the project.

If migrations have not yet been implemented, create the required schema before starting the application. Do not assume that running the Go application automatically creates all tables.

### 6. Start the Backend

If `main.go` is located in the repository root:

```bash
go run .
```

Alternatively, build the application:

```bash
go build -o studio-backend .
./studio-backend
```

The server address and port depend on the application configuration. For local development, a common address is `http://localhost:3000` when the configured port is `3000`.

## Environment Configuration

Example `.env.example`:

```env
APP_NAME=Studio Backend CRM
APP_ENV=development
APP_PORT=3000

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=change_me
DB_NAME=studio_crm
DB_SSLMODE=disable
```

These variables are examples. The actual variable names must match the configuration loader implemented in the project.

### Domains and the superadmin seed

Every user, role and device belongs to a domain. `shutterx` is the root domain; all other domains are its children and are created through `POST /live/api/v1/domains` by the root superadmin. On startup the backend creates `shutterx` (moving any pre-existing users/roles into it) and, if missing, the user `superadmin@shutterx` with the `superadmin` role:

```env
SUPERADMIN_PASSWORD=choose_a_strong_password   # required the first time; min 8 characters
SUPERADMIN_USERNAME=superadmin                 # optional
SUPERADMIN_EMAIL=superadmin@shutterx           # optional
```

Log in with `{"login": "superadmin@shutterx", "password": "..."}` (or `username` + `domain`). Existing users can keep logging in with their real email address, which is looked up in `shutterx`.

**Important:**

- Never commit `.env` to Git.
- Commit `.env.example` with placeholder values only.
- Use strong credentials and appropriate SSL settings in production.
- Keep secrets, signing keys, and database credentials outside source code.

## API Structure

The backend exposes REST API endpoints for the admin panel and other authorized clients.

Example API organization:

| Endpoint group | Purpose |
|---|---|
| `/api/v1/auth` | Authentication |
| `/api/v1/users` | User management |
| `/api/v1/roles` | Role management |
| `/api/v1/permissions` | Permission management |
| `/api/v1/clients` | Client management |
| `/api/v1/bookings` | Booking management |
| `/api/v1/projects` | Project management |
| `/api/v1/services` | Services and packages |
| `/api/v1/payments` | Payment management |
| `/api/v1/reports` | Reports and analytics |

These are proposed route groups, not a guarantee that every endpoint is already implemented. Document actual HTTP methods, request bodies, permissions, and response formats as the APIs are developed.

## Security

Security requirements for the backend include:

- Secure password hashing.
- Authentication middleware for protected routes.
- Server-side role and permission validation.
- Input validation and consistent error handling.
- Parameterized SQL queries to prevent SQL injection.
- Restricted access to sensitive user and payment information.
- Secure handling of uploaded media and files.
- Appropriate CORS configuration.
- HTTPS and secure secret management in production.
- Audit logging for important administrative operations, where required.

## Testing

Run the Go test suite:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

Important test scenarios include:

- Authentication and invalid credentials.
- Role-based access and permission enforcement.
- Client creation and updates.
- Booking creation and scheduling.
- Project assignment and status transitions.
- Payment recording and balance calculations.
- Database errors and input validation.
- Unauthorized access to protected endpoints.

## Deployment

Before deploying to a production server:

1. Configure production environment variables.
2. Provision PostgreSQL and create the application database.
3. Apply database migrations.
4. Build the Go application for the target operating system.
5. Configure the application process and restart policy.
6. Configure a reverse proxy and HTTPS where appropriate.
7. Restrict database network access.
8. Verify authentication, authorization, and critical API workflows.
9. Configure logging, monitoring, and database backups.

Never expose the PostgreSQL port or administrative endpoints unnecessarily to the public internet.

## Git Workflow

Create a feature branch:

```bash
git checkout -b feature/client-management
```

Stage your changes:

```bash
git add .
```

Commit your changes:

```bash
git commit -m "Add client management"
```

Push the branch:

```bash
git push origin feature/client-management
```

Use pull requests and review changes before merging into the main branch.

## Future Enhancements

Potential enhancements include:

- Online client booking.
- Automated booking reminders.
- Studio calendar and staff availability.
- Photo and video delivery tracking.
- Secure client galleries.
- Digital quotations and contracts.
- Payment gateway integration.
- Email and messaging notifications.
- Advanced business reports and analytics.

## License

Specify the applicable license or internal usage policy before distributing the application.

---

**Studio Backend CRM**

*One platform to manage clients, bookings, projects, and operations for photography and videography studios.*
