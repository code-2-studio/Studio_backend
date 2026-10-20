package routes

import (
	"net/http"

	"studio_backend/controllers"
	"studio_backend/middlewares"
	"studio_backend/models"

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

	// Everything below requires a signed-in user; data is scoped to the caller's domain
	authenticated := middlewares.RoleCheckMiddleware()

	// Domains Group (root domain superadmin only)
	domains := live.Group("/v1/domains",
		middlewares.RoleCheckMiddleware(models.SuperAdminRoleName),
		middlewares.RootSuperAdminMiddleware(),
	)
	domains.Get("/", controllers.GetAllDomains)
	domains.Get("/:id", controllers.GetDomainByID)
	domains.Post("/", controllers.CreateDomain)
	domains.Put("/:id", controllers.UpdateDomain)

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

	permissions := live.Group("/v1/permissions", authenticated)
	permissions.Get("/", controllers.GetAllPermissions)
}
