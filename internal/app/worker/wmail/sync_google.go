package wmail

import (
	"context"
	"errors"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (w *WMail) SyncGoogle(ctx context.Context) *errx.MailError {
	newHistoryID, err := w.GoogleData.Client.FetchHistory(ctx, w.GoogleData.LastHistoryID)
	// Gmail history IDs expire. A 404 does not mean the mailbox is gone: reset
	// from the current inbox, persist the new cursor, and resume incremental
	// syncs. Without this recovery every subsequent poll fails forever and the
	// unified inbox never sees new replies.
	if errMail, ok := err.(*errx.MailError); ok && errMail.Code == errx.MailErrorCodeGoogleUnknown(404) {
		newHistoryID, err = w.GoogleData.Client.ResyncInbox(ctx)
	}
	if newHistoryID != 0 {
		if err := w.NewHistoryID(newHistoryID); err != nil {
			w.CaptureError(err)
			return nil
		}

		return nil
	}
	if err != nil {
		var errMail *errx.MailError
		if errors.As(err, &errMail) {
			return errMail
		}

		w.CaptureError(err)

		return nil
	}

	return nil
}

func (w *WMail) NewHistoryID(historyID uint64) error {
	return w.onEvent(models.JobEventTypeHistoryIDUpdate, &models.JobEventHistoryIDUpdate{
		HistoryID: historyID,
	})
}
