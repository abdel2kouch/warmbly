package goog

import (
	"context"
	"time"

	"google.golang.org/api/googleapi"
)

func (c *Client) FetchHistory(ctx context.Context, lastHistoryID uint64) (uint64, error) {
	call := c.srv.Users.History.List("me").MaxResults(500).StartHistoryId(lastHistoryID).Context(ctx) // It does not include the record that has that exact HistoryID

	var newLastHistoryID uint64

	for {
		resp, err := call.Do()
		if err != nil {
			return newLastHistoryID, HandleError(err)
		}

		for _, h := range resp.History {
			for _, m := range h.MessagesAdded {
				// History entries carry only id/threadId/labelIds — no envelope,
				// headers, or body — so hydrate the full message before mapping.
				// A 404 means the message was deleted between the history event
				// and now; skip it rather than failing the whole sync.
				full, ferr := c.srv.Users.Messages.Get("me", m.Message.Id).Format("full").Context(ctx).Do()
				if ferr != nil {
					if gerr, ok := ferr.(*googleapi.Error); ok && gerr.Code == 404 {
						continue
					}
					return newLastHistoryID, HandleError(ferr)
				}

				msg := GmailMessageToEmailData(full)
				if err := c.OnMessageAdd(ctx, msg); err != nil {
					return newLastHistoryID, err
				}
			}
			for _, m := range h.MessagesDeleted {
				if err := c.OnMessageRemove(ctx, m.Message.Id); err != nil {
					return newLastHistoryID, err
				}
			}
			for _, m := range h.LabelsAdded {
				if err := c.OnLabelAdd(ctx, m.Message.Id, m.LabelIds); err != nil {
					return newLastHistoryID, err
				}
			}
			for _, m := range h.LabelsRemoved {
				if err := c.OnLabelRemove(ctx, m.Message.Id, m.LabelIds); err != nil {
					return newLastHistoryID, err
				}
			}
		}
		newLastHistoryID = resp.HistoryId
		if resp.NextPageToken == "" {
			break
		}
		call.PageToken(resp.NextPageToken)
	}

	return newLastHistoryID, nil
}

// ResyncInbox rebuilds the local view from Gmail's current inbox and returns a
// fresh history cursor. Gmail expires old history cursors and responds with a
// 404; continuing to retry that cursor leaves the unified inbox permanently
// stale. A bounded full inbox pass lets us recover without re-authorizing.
func (c *Client) ResyncInbox(ctx context.Context) (uint64, error) {
	// Capture the fresh cursor first. Even if the bounded inbox hydration is
	// partially rate-limited, returning this cursor lets the worker persist it
	// and avoids repeating a many-thousand-message bootstrap every minute.
	profile, err := c.srv.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return 0, HandleError(err)
	}
	call := c.srv.Users.Messages.List("me").LabelIds("INBOX").MaxResults(100)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return profile.HistoryId, HandleError(err)
	}
	for _, m := range resp.Messages {
		full, ferr := c.srv.Users.Messages.Get("me", m.Id).Format("full").Context(ctx).Do()
		if ferr != nil {
			if gerr, ok := ferr.(*googleapi.Error); ok && gerr.Code == 404 {
				continue
			}
			return profile.HistoryId, HandleError(ferr)
		}
		if err := c.OnMessageAdd(ctx, GmailMessageToEmailData(full)); err != nil {
			return profile.HistoryId, err
		}
		select {
		case <-ctx.Done():
			return profile.HistoryId, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return profile.HistoryId, nil
}
