package gcm

import (
	"firebase.google.com/go/v4/messaging"
)

// NewMessage returns a new Message with the specified payload
// and Token(s).
func NewMessage(data map[string]string, tokens ...string) *messaging.MulticastMessage {
	return &messaging.MulticastMessage{Tokens: tokens, Data: data}
}
