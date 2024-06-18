package gcm

import (
	"firebase.google.com/go/v4/messaging"
)

const (
	AndroidNotificationIcon = "ic_notification"
)

// NewMessage returns a new Message with the specified payload
// and Token(s).
func NewMessage(data map[string]string, notification *messaging.Notification, tokens ...string) *messaging.MulticastMessage {
	return &messaging.MulticastMessage{
		Tokens:       tokens,
		Data:         data,
		Notification: notification,
		Android: &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Icon: AndroidNotificationIcon,
			},
		},
	}
}
