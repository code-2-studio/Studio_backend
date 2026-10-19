package jobs

import (
	"time"

	"studio_backend/utils"
)

// RunHealthCheckJob logs periodic system health
func RunHealthCheckJob() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		utils.Logger.Infow("Background health check job executed", "timestamp", time.Now().UTC())
	}
}
