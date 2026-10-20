package middlewares

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ipWindow counts the requests one IP made in the current window.
type ipWindow struct {
	count   int
	resetAt time.Time
}

// RateLimitByIP allows at most max requests per IP in each window and answers 429 after that.
// The counters live in memory, so they reset when the server restarts and are per server instance.
func RateLimitByIP(max int, window time.Duration) fiber.Handler {
	var (
		mu      sync.Mutex
		windows = map[string]*ipWindow{}
	)

	return func(c *fiber.Ctx) error {
		ip := c.IP()
		now := time.Now()

		mu.Lock()
		// drop expired entries so the map cannot grow without bound
		for key, w := range windows {
			if now.After(w.resetAt) {
				delete(windows, key)
			}
		}
		w := windows[ip]
		if w == nil {
			w = &ipWindow{resetAt: now.Add(window)}
			windows[ip] = w
		}
		w.count++
		blocked := w.count > max
		retryAfter := int(w.resetAt.Sub(now).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		mu.Unlock()

		if blocked {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(retryAfter))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status":  false,
				"code":    fiber.StatusTooManyRequests,
				"message": "Too many attempts, try again later",
			})
		}
		return c.Next()
	}
}
