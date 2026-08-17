package goog

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

func HandleError(err error) *errx.MailError {
	if err == nil {
		return nil
	}
	// Google returns an OAuth RetrieveError before an API request is made when
	// a refresh token was revoked or expired. This is not a transient network
	// failure: classify it as an auth error so the consumer disables the mailbox
	// and removes it from campaign rotation until it is reconnected.
	var oauthErr *oauth2.RetrieveError
	if errors.As(err, &oauthErr) && strings.EqualFold(oauthErr.ErrorCode, "invalid_grant") {
		return errx.ErrMailGoogleAuth
	}
	if strings.Contains(strings.ToLower(err.Error()), `oauth2: "invalid_grant"`) {
		return errx.ErrMailGoogleAuth
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch gerr.Code {
		case 401:
			return errx.ErrMailGoogleAuth
		case 402:
			return errx.ErrMailGooglePayment
		case 403:
			msg := strings.ToLower(gerr.Message)
			if strings.Contains(msg, "rate limit") || strings.Contains(msg, "ratelimit") || strings.Contains(msg, "quota") {
				return errx.MError(errx.MailErrorWarning, errx.MailErrorCodeRateLimitExceeded, gerr.Message, errx.MailErrorResolveMethodRetry)
			}
			return errx.ErrMailGoogleForbidden(gerr.Message)
		case 429:
			return errx.MError(errx.MailErrorWarning, errx.MailErrorCodeRateLimitExceeded, gerr.Message, errx.MailErrorResolveMethodRetry)
		default:
			respErr := errx.ErrMailGoogleUnknown(gerr.Code, gerr.Message)
			log.Debug().Err(err).Msg("Google Api Error")
			return respErr
		}
	}

	// Non-API failures (DNS, TLS, timeouts) are transient transport errors.
	// Returning nil here would silently swallow them and leave callers holding
	// a typed-nil *MailError in an error interface.
	log.Debug().Err(err).Msg("Google transport error")
	return errx.ErrMailServerUnreachable
}
