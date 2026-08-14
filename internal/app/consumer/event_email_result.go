package jobs

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/infrastructure/pubsub"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

func (s *JobsService) HandleEmailSentResult(ctx context.Context, result models.SendEmailResult) error {
	if s.CampaignDeliveryRepository == nil {
		return nil
	}
	d, applied, err := s.CampaignDeliveryRepository.FinalizeSuccess(ctx, result)
	if err != nil {
		return fmt.Errorf("finalize campaign delivery success: %w", err)
	}
	if !applied || d == nil {
		return nil
	}
	log.Info().Str("task_id", result.TaskID.String()).Str("campaign_id", d.CampaignID.String()).Msg("campaign delivery confirmed by worker")
	s.publishCampaignDeliveryRealtime(ctx, d, true)
	s.publishConfirmedDeliveryEvents(ctx, d, result)
	return nil
}

func (s *JobsService) HandleEmailFailedResult(ctx context.Context, result models.SendEmailResult) error {
	if s.CampaignDeliveryRepository == nil {
		return nil
	}
	d, applied, err := s.CampaignDeliveryRepository.FinalizeFailure(ctx, result)
	if err != nil {
		return fmt.Errorf("finalize campaign delivery failure: %w", err)
	}
	if !applied || d == nil {
		return nil
	}
	log.Warn().Str("task_id", result.TaskID.String()).Str("campaign_id", d.CampaignID.String()).Msg("campaign delivery rejected by worker")
	s.publishCampaignDeliveryRealtime(ctx, d, false)
	return nil
}

func (s *JobsService) publishConfirmedDeliveryEvents(ctx context.Context, d *repository.CampaignDeliveryContext, result models.SendEmailResult) {
	if s.CampaignRepository == nil || s.ContactRepository == nil || s.EmailRepository == nil {
		return
	}
	campaign, err := s.CampaignRepository.GetByID(ctx, d.CampaignID)
	if err != nil || campaign == nil {
		return
	}
	contact, xerr := s.ContactRepository.GetByID(ctx, d.ContactID)
	if xerr != nil || contact == nil {
		return
	}
	sequence, err := s.CampaignRepository.GetSequenceByID(ctx, d.SequenceID)
	if err != nil || sequence == nil {
		return
	}
	account, xerr := s.EmailRepository.GetByID(ctx, d.EmailAccountID)
	if xerr != nil || account == nil {
		return
	}
	messageID := result.MessageID
	if messageID == "" {
		messageID = result.ProviderMsgID
	}
	task := &repository.Task{
		ID: d.TaskID, TaskType: "campaign", EmailAccountID: d.EmailAccountID,
		Status: "completed", MessageID: messageID, CompletedAt: &result.SentAt,
	}
	if s.Publisher != nil {
		if err := s.Publisher.PublishEmailSent(ctx, task, account, campaign, contact, sequence); err != nil {
			log.Warn().Err(err).Str("task_id", d.TaskID.String()).Msg("failed to publish confirmed email event")
		}
	}
	if s.AdvancedService != nil && campaign.OrganizationID != nil {
		s.AdvancedService.EmitCampaignEvent(ctx, *campaign.OrganizationID, models.WebhookEventCampaignEmailSent, map[string]any{
			"campaign_id": campaign.ID.String(), "contact_id": contact.ID.String(),
			"sequence_id": sequence.ID.String(), "contact_email": contact.Email,
			"from_email": account.Email,
		})
	}
}

func (s *JobsService) publishCampaignDeliveryRealtime(ctx context.Context, d *repository.CampaignDeliveryContext, success bool) {
	if s.StreamingPublisher == nil {
		return
	}
	orgID := ""
	if d.OrganizationID != nil {
		orgID = d.OrganizationID.String()
	}
	status, message, eventType := "failed", "Email delivery failed", pubsub.EventTaskFailed
	if success {
		status, message, eventType = "completed", "Email sent successfully", pubsub.EventTaskCompleted
	}
	s.StreamingPublisher.PublishTaskStatus(ctx, d.UserID, d.TaskID, eventType, message, map[string]string{
		"campaign_id": d.CampaignID.String(), "contact_id": d.ContactID.String(),
	})
	if success {
		progress, total, processed := 0, 0, 0
		if s.CampaignProgressRepository != nil {
			if p, err := s.CampaignProgressRepository.GetCampaignProgress(ctx, d.CampaignID); err == nil && p != nil {
				total, processed = p.TotalContacts, p.EmailsSent
				if total > 0 {
					progress = processed * 100 / total
				}
				s.StreamingPublisher.PublishCampaignProgress(ctx, d.UserID, d.CampaignID, p)
			}
		}
		s.StreamingPublisher.PublishEmailSent(ctx, &pubsub.TaskProgressEvent{
			BaseEvent: pubsub.BaseEvent{UserID: d.UserID}, OrgID: orgID,
			CampaignID: d.CampaignID.String(), TaskID: d.TaskID.String(), Status: status,
			ContactID: d.ContactID.String(), ContactEmail: d.ContactEmail, ContactName: d.ContactName,
			SequenceID: d.SequenceID.String(), SequenceName: d.SequenceName, SequenceIndex: d.SequenceIndex,
			Progress: progress, TotalContacts: total, ProcessedCount: processed,
		})
	}
	s.StreamingPublisher.PublishCampaignEvent(ctx, &pubsub.CampaignEvent{
		BaseEvent: pubsub.BaseEvent{EventType: pubsub.EventCampaignUpdated, UserID: d.UserID},
		OrgID:     orgID, CampaignID: d.CampaignID.String(), Name: d.CampaignName,
	})
}
