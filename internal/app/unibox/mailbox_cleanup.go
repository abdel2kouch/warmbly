package unibox

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

const mailboxCleanupBatchSize = 1000

// PreviewMailboxCleanup validates the selected mailboxes and calculates what a
// confirmed cleanup would queue. It never moves or deletes any message.
func (s *uniboxService) PreviewMailboxCleanup(ctx context.Context, orgID uuid.UUID, mailboxIDs []uuid.UUID) (*models.MailboxCleanupPreview, *errx.Error) {
	mailboxIDs = uniqueMailboxIDs(mailboxIDs)
	if len(mailboxIDs) == 0 {
		return nil, errx.New(errx.BadRequest, "select at least one mailbox")
	}
	if s.emailRepository == nil || s.uniboxRepository == nil {
		return nil, errx.New(errx.Internal, "mailbox cleanup service is not configured")
	}

	unsupported := 0
	for _, mailboxID := range mailboxIDs {
		account, xerr := s.emailRepository.GetByID(ctx, mailboxID)
		if xerr != nil || account == nil {
			return nil, errx.New(errx.NotFound, "mailbox not found")
		}
		if account.OrganizationID == nil || *account.OrganizationID != orgID {
			return nil, errx.New(errx.Forbidden, "mailbox is outside this organization")
		}
		if account.Provider != string(models.InboxProviderGoogle) || account.WorkerID == nil {
			unsupported++
		}
	}

	preview, err := s.uniboxRepository.PreviewMailboxCleanup(ctx, orgID, mailboxIDs)
	if err != nil {
		sentry.CaptureException(err)
		return nil, errx.InternalError()
	}
	preview.UnsupportedMailboxes = unsupported
	return preview, nil
}

// CleanupMailboxes queues selected Gmail messages for Trash after the caller
// has reviewed the preview. Conversations that contain replies from contacted
// campaign leads are excluded by the repository on both planning and deletion.
func (s *uniboxService) CleanupMailboxes(ctx context.Context, orgID uuid.UUID, mailboxIDs []uuid.UUID) (*models.MailboxCleanupResult, *errx.Error) {
	preview, xerr := s.PreviewMailboxCleanup(ctx, orgID, mailboxIDs)
	if xerr != nil {
		return nil, xerr
	}
	if preview.UnsupportedMailboxes > 0 {
		return nil, errx.New(errx.BadRequest, "mailbox cleanup currently requires active Gmail mailboxes")
	}
	if s.publisher == nil {
		return nil, errx.New(errx.Internal, "mailbox cleanup service is not configured")
	}

	mailboxIDs = uniqueMailboxIDs(mailboxIDs)
	targets, err := s.uniboxRepository.MailboxCleanupTargets(ctx, orgID, mailboxIDs)
	if err != nil {
		sentry.CaptureException(err)
		return nil, errx.InternalError()
	}

	byMailbox := make(map[uuid.UUID][]string)
	for _, target := range targets {
		byMailbox[target.EmailID] = append(byMailbox[target.EmailID], target.GmailID)
	}
	for _, mailboxID := range mailboxIDs {
		gmailIDs := byMailbox[mailboxID]
		if len(gmailIDs) == 0 {
			continue
		}
		account, xerr := s.emailRepository.GetByID(ctx, mailboxID)
		if xerr != nil || account == nil || account.WorkerID == nil {
			return nil, errx.New(errx.Internal, "couldn't queue mailbox cleanup")
		}
		for start := 0; start < len(gmailIDs); start += mailboxCleanupBatchSize {
			end := start + mailboxCleanupBatchSize
			if end > len(gmailIDs) {
				end = len(gmailIDs)
			}
			if err := s.publisher.PublishMailboxCleanup(ctx, *account.WorkerID, &models.MailboxCleanupAction{
				EmailID:  mailboxID,
				GmailIDs: gmailIDs[start:end],
			}); err != nil {
				sentry.CaptureException(err)
				return nil, errx.New(errx.Internal, fmt.Sprintf("couldn't queue cleanup for %s", account.Email))
			}
		}
	}

	// The local cache mirrors the inbox. Removing only the queued rows prevents
	// stale conversations while Gmail applies the asynchronous Trash operation.
	if err := s.uniboxRepository.DeleteMailboxCleanupCache(ctx, orgID, mailboxIDs); err != nil {
		sentry.CaptureException(err)
		return nil, errx.InternalError()
	}
	return &models.MailboxCleanupResult{
		SelectedMailboxes: preview.SelectedMailboxes,
		QueuedForTrash:    int64(len(targets)),
		PreservedThreads:  preview.PreservedThreads,
		PreservedMessages: preview.PreservedMessages,
	}, nil
}

func uniqueMailboxIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
