package goog

import (
	"fmt"
	"testing"

	"github.com/warmbly/warmbly/internal/errx"
	"google.golang.org/api/googleapi"
)

func TestHandleErrorClassifiesWrappedGoogleRateLimit(t *testing.T) {
	wrapped := fmt.Errorf("send message failed: %w", &googleapi.Error{
		Code: 429, Message: "User-rate limit exceeded. Retry later.",
	})
	got := HandleError(wrapped)
	if got == nil || got.Code != errx.MailErrorCodeRateLimitExceeded {
		t.Fatalf("expected RATE_LIMIT_EXCEEDED, got %#v", got)
	}
	if got.Message != "User-rate limit exceeded. Retry later." {
		t.Fatalf("provider retry detail was lost: %q", got.Message)
	}
}

func TestHandleErrorClassifies403RateLimitReason(t *testing.T) {
	got := HandleError(&googleapi.Error{Code: 403, Message: "userRateLimitExceeded quota reached"})
	if got == nil || got.Code != errx.MailErrorCodeRateLimitExceeded {
		t.Fatalf("expected RATE_LIMIT_EXCEEDED, got %#v", got)
	}
}
