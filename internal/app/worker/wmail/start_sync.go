package wmail

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// StartSyncWorker runs a periodic mail sync loop until the context is cancelled.
// It dispatches to the right provider (Google history-based or IMAP poll-based)
// and keeps the inbox up to date by emitting JobEventTypeNewEmail and other
// events whenever changes are detected on the upstream mail server.
func (w *WMail) StartSyncWorker(ctx context.Context) {
	interval := ImapCheckInterval
	if w.EmailType == models.InboxProviderGoogle {
		// Google polling can be slightly slower because the API is more efficient
		// and rate-limited.
		interval = 1 * time.Minute
	}

	// Run immediately, then adapt the delay. Gmail user-rate limits require a
	// real cooldown; retrying every minute can keep an account permanently
	// throttled, especially while recovering an expired history cursor.
	delay := time.Duration(0)
	consecutiveFailures := 0
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		mailErr := w.syncOnce(ctx)
		if mailErr == nil {
			consecutiveFailures = 0
			delay = interval
			continue
		}
		consecutiveFailures++
		switch mailErr.Code {
		case errx.MailErrorCodeRateLimitExceeded, errx.MailErrorCodeQuotaExceeded, errx.MailErrorCodeSendingTooFast:
			delay = 15 * time.Minute
		case errx.MailErrorCodeGoogleAuth, errx.MailErrorCodeAuthenticationFailed, errx.MailErrorCodeAuthorizationFailed:
			delay = 30 * time.Minute
		default:
			delay = time.Duration(1<<min(consecutiveFailures, 6)) * time.Minute
			if delay < interval {
				delay = interval
			}
		}
	}
}

// syncOnce runs one sync pass, containing panics: the worker is multi-tenant,
// so one mailbox's bad server response must not take down every other
// account's sync and send loops.
func (w *WMail) syncOnce(ctx context.Context) (result *errx.MailError) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("mail sync panic: %v", r)
			w.CaptureError(err)
			log.Error().Err(err).Str("email_id", w.ID.String()).Msg("mail sync panicked")
			result = errx.ErrMailServerUnreachable
		}
	}()
	if err := w.SyncMail(ctx); err != nil {
		w.CaptureError(err)
		log.Warn().Err(err).Str("email_id", w.ID.String()).Msg("mail sync error")
		return err
	}
	return nil
}
