package gcm

import (
	"context"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"github.com/appleboy/go-fcm"
	"math/rand"
	"time"
)

const (
	// Initial delay before first retry, without jitter.
	backoffInitialDelay = 1000
	// Maximum delay before a retry.
	maxBackoffDelay = 1024000
)

// Errors

type JSONParseError struct{ error }
type UnauthorizedError struct{ error }
type UnknownError struct{ error }

const (
	ResponseErrorMissingRegistration = "MissingRegistration"
	ResponseErrorInvalidRegistration = "InvalidRegistration"
	ResponseErrorMismatchSenderID    = "MismatchSenderId"
	ResponseErrorNotRegistered       = "NotRegistered"
	ResponseErrorMessageTooBig       = "MessageTooBig"
	ResponseErrorInvalidDataKey      = "InvalidDataKey"
	ResponseErrorInvalidTTL          = "InvalidTtl"
	ResponseErrorUnavailable         = "Unavailable"
	ResponseErrorInternalServerError = "InternalServerError"
	ResponseErrorInvalidPackageName  = "InvalidPackageName"
)

// Sender abstracts the interaction between the application server and the
// GCM server. The developer must obtain an API key from the Google APIs
// Console page and pass it to the Sender so that it can perform authorized
// requests on the application server's behalf. To send a message to one or
// more devices use the Sender's Send or SendNoRetry methods.
//
// If Sender Client is nil, checkSender will automatically initialize a Client
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		c := appengine.NewContext(r)
//		sender := &gcm.Sender{CredentialsJson: key}
//
//		/* ... */
//	}
type Sender struct {
	CredentialsJson string
	Client          *fcm.Client
}

// SendNoRetry sends a message to the FCM server without retrying in case of
// service unavailability. A non-nil error is returned if a non-recoverable
// error occurs.
// If msg is a valid MulticastMessage, then the failed tokens will also be returned.
func (s *Sender) SendNoRetry(msg *messaging.MulticastMessage) (*messaging.BatchResponse, []string, error) {
	// Note that failed tokens returns as nil, since we cannot guarantee that msg.Tokens exists
	if err := checkMessage(msg); err != nil {
		return nil, nil, err
	} else if err = checkSender(s); err != nil {
		return nil, msg.Tokens, err
	}
	resp, err := s.Client.SendMulticast(context.Background(), msg)
	if err != nil {
		return resp, msg.Tokens, err
	}

	// Collect failed tokens
	var failedTokens []string
	for idx, r := range resp.Responses {
		if !r.Success {
			failedTokens = append(failedTokens, msg.Tokens[idx])
		}
	}

	return resp, failedTokens, nil
}

// Send sends a message to the GCM server, retrying in case of service
// unavailability. A non-nil error is returned if a non-recoverable
// error occurs (i.e. if the response status is not "200 OK").
//
// Note that messages are retried using exponential backoff, and as a
// result, this method may block for several seconds.
func (s *Sender) Send(msg *messaging.MulticastMessage, retries int) (*messaging.BatchResponse, []string, error) {
	// Note that failed tokens returns as nil, since we cannot guarantee that msg.Tokens exists
	if err := checkMessage(msg); err != nil {
		return nil, nil, err
	} else if err = checkSender(s); err != nil {
		return nil, msg.Tokens, err
	} else if retries < 0 {
		return nil, msg.Tokens, errors.New("retries must not be negative")
	}

	// Send the message for the first time.
	resp, failedTokens, err := s.SendNoRetry(msg)
	if err != nil {
		return nil, failedTokens, err
	} else if resp.FailureCount == 0 || retries == 0 {
		return resp, failedTokens, nil
	}

	// One or more messages failed to send.
	regIDs := msg.Tokens
	allResults := make(map[string]*messaging.SendResponse, len(regIDs))
	backoff := backoffInitialDelay
	for i := 0; updateStatus(msg, resp, allResults) > 0 && i < retries; i++ {
		sleepTime := backoff/2 + rand.Intn(backoff)
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		backoff = min(2*backoff, maxBackoffDelay)
		if resp, failedTokens, err = s.SendNoRetry(msg); err != nil {
			msg.Tokens = regIDs
			return nil, failedTokens, err
		}
	}

	// Restore the original list of registration tokens.
	msg.Tokens = regIDs

	// Create a final BatchResponse and list of failed tokens.
	finalResponses := make([]*messaging.SendResponse, len(regIDs))
	for i, token := range regIDs {
		if result, ok := allResults[token]; ok {
			finalResponses[i] = result
		} else {
			finalResponses[i] = &messaging.SendResponse{
				Success: false,
				Error:   errors.New("unknown error"),
			}
		}
	}

	finalBatchResponse := &messaging.BatchResponse{
		SuccessCount: resp.SuccessCount,
		FailureCount: resp.FailureCount,
		Responses:    finalResponses,
	}

	return finalBatchResponse, failedTokens, nil
}

// updateStatus updates the status of the messages sent to devices and
// returns the number of recoverable errors that could be retried.
func updateStatus(msg *messaging.MulticastMessage, resp *messaging.BatchResponse, allResults map[string]*messaging.SendResponse) int {
	unsentRegIDs := make([]string, 0, resp.FailureCount)
	for i := 0; i < len(resp.Responses); i++ {
		regID := msg.Tokens[i]
		allResults[regID] = resp.Responses[i]
		if resp.Responses[i].Error != nil && isRecoverableError(resp.Responses[i].Error) {
			unsentRegIDs = append(unsentRegIDs, regID)
		}
	}
	msg.Tokens = unsentRegIDs
	return len(unsentRegIDs)
}

// isRecoverableError checks if the error is a recoverable error.
// This is under the assumption that Legacy and HTTP V1 + SDK return
// the same errors.
// For more info, check out:
// https://firebase.google.com/docs/cloud-messaging/send-message#rest
func isRecoverableError(err error) bool {
	return err.Error() == "Unavailable"
}

// checkSender returns an error if the sender is not well-formed and
// initializes a zeroed fcm.Client if one has not been provided.
func checkSender(sender *Sender) error {
	if sender.CredentialsJson == "" {
		return errors.New("the sender's credentials data must not be empty")
	}
	if sender.Client == nil {
		client, err := initFCMClient(sender)
		if err != nil {
			return err
		}
		sender.Client = client
	}
	return nil
}

func initFCMClient(sender *Sender) (*fcm.Client, error) {
	ctx := context.Background()
	client, err := fcm.NewClient(
		ctx,
		fcm.WithCredentialsJSON([]byte(sender.CredentialsJson)),
	)
	return client, err
}

// checkMessage returns an error if the message is not well-formed.
func checkMessage(msg *messaging.MulticastMessage) error {
	if msg == nil {
		return errors.New("the message must not be nil")
	} else if msg.Tokens == nil {
		return errors.New("the message's Tokens field must not be nil")
	} else if len(msg.Tokens) == 0 {
		return errors.New("the message must specify at least one Token")
	} else if len(msg.Tokens) > 500 {
		return errors.New("the message may specify at most 500 registration IDs")
	}
	return nil
}
