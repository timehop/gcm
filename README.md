gcm
===

This is a modified version of the gcm library @alexjlockwood with some changes suggested by @timehop.  However the message responses are quite different as the original library didn't make it feasible to distinguish between different GCM error responses, and didn't handle the 'Retry-After' headers that Google say are mandatory (or they may block your account).

The Android SDK provides a nice convenience library ([com.google.android.gcm.server](http://developer.android.com/reference/com/google/android/gcm/server/package-summary.html)) that greatly simplifies the interaction between Java-based application servers and Google's GCM servers. However, Google has not provided much support for application servers implemented in languages other than Java, specifically those written in the Go programming language. The `gcm` package helps to fill in this gap, providing a simple interface for sending GCM messages and automatically retrying requests in case of service unavailability.

Documentation: http://godoc.org/github.com/johngb/gcm

Getting Started
---------------

To install gcm, use `go get`:

```bash
go get github.com/johngb/gcm
```

Import gcm with the following:

```go
import "github.com/johngb/gcm"
```

Sample Usage
------------

Here is a quick sample illustrating how to send a message to the GCM server:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/johngb/gcm"
)

func main() {
	// Create the message to be sent.
	data := map[string]interface{}{"score": "5x1", "time": "15:10"}
	regIDs := []string{"4", "8", "15", "16", "23", "42"}
	msg := gcm.NewMessage(data, regIDs...)

	// Create a Sender to send the message.
	sender := &gcm.Sender{ApiKey: "sample_api_key"}

	// Send the message and receive the response after at most two retries.
	response, httpErr := sender.Send(msg, 2)
	if httpErr.Err != nil {
		switch httpErr.StatusCode{
		case http.StatusBadRequest:
			...
			// handle bad requests as needed
		case http.StatusInternalServerError
			...
			// handle internal server errors as needed, keeping to the httpErr.RetryAfter if you retry
		...
		}
		fmt.Println("Failed to send message:", err)
		return
	}

	/* ... */
}
```

Note for Google AppEngine users
-------------------------------

If your application server runs on Google AppEngine, you must import the `appengine/urlfetch` package and create the `Sender` as follows:

```go
package sample

import (
	"appengine"
	"appengine/urlfetch"

	"github.com/johngb/gcm"
)

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	sender := &gcm.Sender{ApiKey: "sample_api_key", HTTP: client}

	/* ... */
}        
```
