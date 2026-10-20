package routes

import (
	"net/http"
	"time"

	"studio_backend/controllers"
	"studio_backend/middlewares"

	"github.com/gofiber/fiber/v2"
)

// Setup registers all API routes onto the Fiber app
func Setup(app *fiber.App) {
	// Welcome / Health check route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"code":    http.StatusOK,
			"message": "Welcome to Studio CRM Backend API",
			"status":  true,
		})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"code":    http.StatusOK,
			"status":  "healthy",
			"message": "Studio CRM Backend is running smoothly",
		})
	})

	DevSetup(app)
	LiveSetup(app)
}

// DevSetup registers development routes
func DevSetup(app *fiber.App) {
	dev := app.Group("/dev/api")

	v1 := dev.Group("/v1")
	v1.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  true,
			"message": "Dev API running",
		})
	})
}

// LiveSetup registers production / live API routes
func LiveSetup(app *fiber.App) {
	live := app.Group("/live/api")

	// Auth Group
	auth := live.Group("/v1/auth")
	auth.Post("/register", controllers.Register)
	auth.Post("/login", controllers.Login)
	auth.Post("/refresh", controllers.RefreshToken)
	auth.Post("/logout", controllers.Logout)

	// First-run setup (public, but needs SETUP_KEY and only works until a superadmin exists).
	// Rate limited so the key cannot be guessed.
	setup := live.Group("/v1/setup")
	setup.Get("/status", controllers.SetupStatus)
	setup.Post("/", middlewares.RateLimitByIP(5, time.Minute), controllers.Setup)

	// Everything below requires a signed-in user; data is scoped to the caller's domain
	authenticated := middlewares.RoleCheckMiddleware()

	// Domains Group: reading is scoped to the caller's domain tree; creating/editing is shutterx superadmin only
	domains := live.Group("/v1/domains", authenticated)
	domains.Get("/", controllers.GetAllDomains)
	domains.Get("/:id", controllers.GetDomainByID)
	domains.Post("/", middlewares.RootSuperAdminMiddleware(), controllers.CreateDomain)
	domains.Put("/:id", middlewares.RootSuperAdminMiddleware(), controllers.UpdateDomain)

	// Studio onboarding: any signed-in user creates a studio (a new domain) below their own domain
	studios := live.Group("/v1/studios", authenticated)
	studios.Post("/", controllers.OnboardStudio)
	studios.Post("/check-domain", controllers.CheckStudioDomain)
	studios.Get("/:id", controllers.GetStudio)
	studios.Put("/:id", controllers.UpdateStudio)

	// Users Group
	users := live.Group("/v1/users", authenticated)
	users.Get("/", controllers.GetAllUsers)
	users.Get("/:id", controllers.GetUserByID)
	users.Post("/", controllers.CreateUser)
	users.Put("/:id", controllers.UpdateUser)
	users.Delete("/:id", controllers.DeleteUser)

	// Roles & Permissions Group
	roles := live.Group("/v1/roles", authenticated)
	roles.Get("/", controllers.GetAllRoles)
	roles.Get("/:id", controllers.GetRoleByID)
	roles.Post("/", controllers.CreateRole)
	roles.Post("/assign", controllers.AssignRole)
	roles.Put("/permissions", controllers.SetRolePermissions)
	roles.Post("/list", controllers.ListRoles)
	roles.Put("/", controllers.UpdateRole)

	permissions := live.Group("/v1/permissions", authenticated)
	// Every permission endpoint takes its parameters in the JSON body, never in the URL
	permissions.Get("/", controllers.GetAllPermissions)    // what my domain can see
	permissions.Post("/view", controllers.ViewPermissions) // one code, or another domain's view (root)
	// Any signed-in user can create permissions (owned by their domain) and edit their own domain's
	permissions.Post("/", controllers.CreatePermissions)
	permissions.Put("/", controllers.UpdatePermission)
	// Any signed-in user can share their own domain's permissions with another domain
	permissions.Put("/share", controllers.SharePermissions)
}
