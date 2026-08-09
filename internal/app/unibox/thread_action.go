package unibox

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

const (
	threadActionArchive    = "archive"
	threadActionTrash      = "trash"
	threadActionMarkRead   = "mark_read"
	threadActionMarkUnread = "mark_unread"
)

// ThreadAction sends a provider-backed command to every Gmail message in a
// thread. Archive/trash also remove the cached conversation immediately, so a
// successful command never leaves a stale row visible while Gmail history sync
// catches up. The source mail remains intact in Gmail (archive) or Trash.
func (s *uniboxService) ThreadAction(ctx context.Context, orgID uuid.UUID, threadID, action string) *errx.Error {
	if threadID == "" {
		return errx.New(errx.BadRequest, "thread_id is required")
	}
	switch action {
	case threadActionArchive, threadActionTrash, threadActionMarkRead, threadActionMarkUnread:
	default:
		return errx.New(errx.BadRequest, "unsupported thread action")
	}
	if s.emailRepository == nil || s.publisher == nil {
		return errx.New(errx.Internal, "inbox action service is not configured")
	}

	targets, err := s.uniboxRepository.ThreadActionTargets(ctx, orgID, threadID)
	if err != nil {
		sentry.CaptureException(err)
		return errx.InternalError()
	}
	if len(targets) == 0 {
		return errx.New(errx.NotFound, "thread not found")
	}

	for _, target := range targets {
		account, xerr := s.emailRepository.GetByID(ctx, target.EmailID)
		if xerr != nil || account == nil {
			return errx.New(errx.NotFound, "mailbox not found")
		}
		if account.OrganizationID == nil || *account.OrganizationID != orgID {
			return errx.New(errx.Forbidden, "mailbox is outside this organization")
		}
		if account.Provider != string(models.InboxProviderGoogle) || account.WorkerID == nil {
			return errx.New(errx.BadRequest, "this inbox action currently requires an active Gmail mailbox")
		}
		if err := s.publisher.PublishWarmupAction(ctx, *account.WorkerID, &models.WarmupEmailAction{
			EmailID: target.EmailID,
			GmailID: target.GmailID,
			Actions: []string{action},
		}); err != nil {
			sentry.CaptureException(err)
			return errx.New(errx.Internal, fmt.Sprintf("couldn't queue %s action", action))
		}
	}

	if action == threadActionArchive || action == threadActionTrash {
		if err := s.uniboxRepository.DeleteThreadForOrg(ctx, orgID, threadID); err != nil {
			sentry.CaptureException(err)
			return errx.InternalError()
		}
	}
	return nil
}
