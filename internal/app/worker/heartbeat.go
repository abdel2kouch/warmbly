package worker

import (
	"context"
	"log"
	"time"
)

func (s *WorkerService) Heartbeat(ctx context.Context) {
	// Register liveness before the first interval elapses. Without this, a
	// freshly started worker is invisible to the consumer for 90 seconds and
	// can be treated as dead before it is eligible for reassignment. This is
	// especially harmful to a self-hosted single-worker deployment: campaigns
	// can be routed to a stale worker topic during that window.
	if err := s.heartbeat(ctx); err != nil {
		log.Println("Failed to do initial heartbeat", err)
	}

	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping heartbeat for", s.ID)
			return
		case <-ticker.C:
			if err := s.heartbeat(ctx); err != nil {
				log.Println("Failed to do heartbeat", err)
			}
		}
	}
}
