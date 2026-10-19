package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"studio_backend/config"
	"studio_backend/db"
	"studio_backend/middlewares"
	"studio_backend/routes"
	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	_ = godotenv.Load()
	cfg := config.Load()

	// Initialize Fiber application
	app := fiber.New(fiber.Config{
		AppName:      "Studio CRM Backend v1.0",
		ServerHeader: "Studio-Backend",
	})

	// Global Middlewares
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:3001,http://localhost:5173,http://127.0.0.1:3000,http://127.0.0.1:5173",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowCredentials: true,
	}))
	app.Use(middlewares.ZapLogger())

	// Connect to Database
	utils.Info("Connecting to database...")
	db.Connect()
	utils.Info("Database initialization completed")

	// Register Routes
	routes.Setup(app)

	// Start server in background
	
	// Start server on the host-provided port
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}
	if port == "" {
		port = "8080"
	}

	addr := "0.0.0.0:" + port
	utils.Info("Server starting", "address", addr, "env", cfg.Env)

	go func() {
		if err := app.Listen(addr); err != nil {
			utils.Logger.Errorf("Server failed to start: %v", err)
			os.Exit(1)
		}
	}()

	gracefulShutdown(app)
}

func gracefulShutdown(app *fiber.App) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	utils.Info("Shutting down gracefully...")

	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		utils.Logger.Errorw("Error shutting down Fiber server", "error", err)
	}

	db.CloseDB()

	time.Sleep(1 * time.Second)
	utils.Info("Shutdown complete")
}
