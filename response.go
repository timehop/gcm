package gcm

// Response represents the GCM server's response to the application
// server's sent message. See the documentation for GCM Architectural
// Overview for more information:
// http://developer.android.com/google/gcm/gcm.html#send-msg
type Response struct {

	// Unique ID (number) identifying the multicast message
	MulticastID int64 `json:"multicast_id"`

	// Number of messages that were processed without an error
	Success int `json:"success"`

	// Number of messages that could not be processed
	Failure int `json:"failure"`

	// Number of results that contain a canonical registration ID
	CanonicalIDs int `json:"canonical_ids"`

	// Array of objects representing the status of the messages processed
	Results []Result `json:"results"`
}

// Result represents the status of a processed message.
type Result struct {

	// String representing the message when it was successfully processed
	MessageID string `json:"message_id"`

	// If set, means that GCM processed the message but it has another
	// canonical registration ID for that device, so sender should replace the
	// IDs on future requests
	RegistrationID string `json:"registration_id"`

	// String describing an error that occurred while processing the message
	// for that recipient
	Error string `json:"error"`
}
