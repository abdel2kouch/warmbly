package worker

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/models"
)

// HandleMailboxCleanup moves a pre-screened batch of Gmail messages to Trash.
// The inbox service excludes campaign-lead reply conversations before it ever
// publishes this event; workers only execute the requested provider action.
func (w *WorkerService) HandleMailboxCleanup(ctx context.Context, action models.MailboxCleanupAction) error {
	if action.EmailID == uuid.Nil || len(action.GmailIDs) == 0 {
		return nil
	}
	w.mailManager.RLock()
	mail, exists := w.mailManager.Emails[action.EmailID]
	w.mailManager.RUnlock()
	if !exists || mail.GoogleData == nil || mail.GoogleData.Client == nil {
		log.Warn().Str("email_id", action.EmailID.String()).Msg("Gmail mailbox unavailable for cleanup")
		return nil
	}
	if err := mail.GoogleData.Client.TrashMessages(ctx, action.GmailIDs); err != nil {
		log.Error().Err(err).Str("email_id", action.EmailID.String()).Int("messages", len(action.GmailIDs)).Msg("failed to move mailbox cleanup batch to Trash")
		return err
	}
	log.Info().Str("email_id", action.EmailID.String()).Int("messages", len(action.GmailIDs)).Msg("moved mailbox cleanup batch to Gmail Trash")
	return nil
}
