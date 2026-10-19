package middlewares

import (
	"time"

	"studio_backend/utils"

	"github.com/gofiber/fiber/v2"
)

// ZapLogger is a Fiber middleware that logs HTTP requests using Zap.
func ZapLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		latency := time.Since(start).String()
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		ip := c.IP()

		fields := []interface{}{
			"status", status,
			"method", method,
			"path", path,
			"ip", ip,
			"latency", latency,
		}

		if err != nil {
			fields = append(fields, "error", err.Error())
			utils.Logger.Errorw("API Request Failed", fields...)
			return err
		}

		utils.Logger.Infow("API Request Successful", fields...)
		return nil
	}
}
