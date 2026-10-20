package middlewares

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func limitedApp(max int, window time.Duration) *fiber.App {
	app := fiber.New()
	app.Post("/x", RateLimitByIP(max, window), func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func status(t *testing.T, app *fiber.App) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("POST", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestRateLimitBlocksAfterTheLimit(t *testing.T) {
	app := limitedApp(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if got := status(t, app); got != 200 {
			t.Fatalf("request %d should pass, got %d", i, got)
		}
	}
	if got := status(t, app); got != 429 {
		t.Fatalf("4th request should be blocked, got %d", got)
	}
}

func TestRateLimitWindowResets(t *testing.T) {
	app := limitedApp(1, 50*time.Millisecond)
	if status(t, app) != 200 || status(t, app) != 429 {
		t.Fatalf("second request in the window should be blocked")
	}
	time.Sleep(80 * time.Millisecond)
	if got := status(t, app); got != 200 {
		t.Fatalf("a new window should allow requests again, got %d", got)
	}
}
