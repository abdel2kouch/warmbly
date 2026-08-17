package repository

import (
	"testing"
	"time"
)

func TestCampaignRetryBackoff(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 1, want: 5 * time.Minute},
		{failures: 2, want: 20 * time.Minute},
		{failures: 3, want: 0},
		{failures: 4, want: 0},
	}

	for _, tt := range tests {
		t.Run(time.Duration(tt.failures).String(), func(t *testing.T) {
			if got := campaignRetryBackoff(tt.failures); got != tt.want {
				t.Fatalf("campaignRetryBackoff(%d) = %s, want %s", tt.failures, got, tt.want)
			}
		})
	}
}
