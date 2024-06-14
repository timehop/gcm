package gcm

import (
	"context"
	"encoding/json"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/appleboy/go-fcm"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testResponse struct {
	StatusCode int
	Response   *messaging.BatchResponse
}

// MockTokenSource is a TokenSource implementation that can be used for testing.
type MockTokenSource struct {
	AccessToken string
}

// Token returns the test token associated with the TokenSource.
func (ts *MockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: ts.AccessToken}, nil
}

func getMockClient(server *httptest.Server) (*fcm.Client, error) {
	return fcm.NewClient(
		context.Background(),
		fcm.WithEndpoint(server.URL),
		fcm.WithProjectID("test"),
		fcm.WithTokenSource(&MockTokenSource{AccessToken: "test-token"}),
	)

}

func startTestServer(t *testing.T, responses ...*testResponse) *httptest.Server {
	i := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		if i >= len(responses) {
			t.Fatalf("server received %d requests, expected %d", i+1, len(responses))
		}
		resp := responses[i]
		status := resp.StatusCode
		if status == 0 || status == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			respBytes, _ := json.Marshal(resp.Response)
			fmt.Fprint(w, string(respBytes))
		} else {
			w.WriteHeader(status)
		}
		i++
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	return server
}

func TestSendNoRetryInvalidApiKey(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{CredentialsJson: ""}
	if _, _, err := sender.SendNoRetry(&messaging.MulticastMessage{Tokens: []string{"1"}}); err == nil {
		t.Fatal("test should fail when sender's CredentialsJson is \"\"")
	}
}

func TestSendInvalidApiKey(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{CredentialsJson: ""}
	if _, _, err := sender.Send(&messaging.MulticastMessage{Tokens: []string{"1"}}, 0); err == nil {
		t.Fatal("test should fail when sender's CredentialsJson is \"\"")
	}
}

func TestSendNoRetryInvalidMessage(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{CredentialsJson: "test"}
	if _, _, err := sender.SendNoRetry(nil); err == nil {
		t.Fatal("test should fail when message is nil")
	}
	if _, _, err := sender.SendNoRetry(&messaging.MulticastMessage{}); err == nil {
		t.Fatal("test should fail when message Tokens field is nil")
	}
	if _, _, err := sender.SendNoRetry(&messaging.MulticastMessage{Tokens: []string{}}); err == nil {
		t.Fatal("test should fail when message Tokens field is an empty slice")
	}
	if _, _, err := sender.SendNoRetry(&messaging.MulticastMessage{Tokens: make([]string, 501)}); err == nil {
		t.Fatal("test should fail when more than 500 Tokens are specified")
	}
}

func TestSendInvalidMessage(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{CredentialsJson: "test"}
	if _, _, err := sender.Send(nil, 0); err == nil {
		t.Fatal("test should fail when message is nil")
	}
	if _, _, err := sender.Send(&messaging.MulticastMessage{}, 0); err == nil {
		t.Fatal("test should fail when message Tokens field is nil")
	}
	if _, _, err := sender.Send(&messaging.MulticastMessage{Tokens: []string{}}, 0); err == nil {
		t.Fatal("test should fail when message Tokens field is an empty slice")
	}
	if _, _, err := sender.Send(&messaging.MulticastMessage{Tokens: make([]string, 1001)}, 0); err == nil {
		t.Fatal("test should fail when more than 1000 Tokens are specified")
	}

}

func TestSendNoRetrySuccess(t *testing.T) {
	server := startTestServer(t, &testResponse{Response: &messaging.BatchResponse{}})
	defer server.Close()
	client, _ := getMockClient(server)
	sender := &Sender{CredentialsJson: "test", Client: client}
	msg := NewMessage(map[string]string{"key": "value"}, "1")
	if _, _, err := sender.SendNoRetry(msg); err != nil {
		t.Fatalf("test failed with error: %s", err)
	}
}

func TestSendNoRetryNonrecoverableFailure(t *testing.T) {
	server := startTestServer(t, &testResponse{StatusCode: http.StatusBadRequest})
	defer server.Close()
	sender := &Sender{CredentialsJson: "test"}
	msg := NewMessage(map[string]string{"key": "value"}, "1")
	if _, _, err := sender.SendNoRetry(msg); err == nil {
		t.Fatal("test expected non-recoverable error")
	}
}

func TestSendSuccess(t *testing.T) {
	server := startTestServer(t,
		&testResponse{Response: &messaging.BatchResponse{FailureCount: 1, Responses: []*messaging.SendResponse{{Error: errors.New("Unavailable")}}}},
		&testResponse{Response: &messaging.BatchResponse{FailureCount: 1, Responses: []*messaging.SendResponse{{Error: errors.New("Unavailable")}}}},
	)
	defer server.Close()
	client, _ := getMockClient(server)
	sender := &Sender{CredentialsJson: "test", Client: client}
	msg := NewMessage(map[string]string{"key": "value"}, "1")
	resp, _, err := sender.Send(msg, 1)
	if err != nil || resp.SuccessCount != 1 {
		t.Fatal("send should return response with one success")
	}
}

func TestSendOneRetryNonrecoverableFailure(t *testing.T) {
	server := startTestServer(t,
		&testResponse{Response: &messaging.BatchResponse{FailureCount: 1, Responses: []*messaging.SendResponse{{Error: errors.New("Unavailable")}}}},
		&testResponse{StatusCode: http.StatusBadRequest},
	)
	defer server.Close()
	sender := &Sender{CredentialsJson: "test"}
	msg := NewMessage(map[string]string{"key": "value"}, "1")
	if _, _, err := sender.Send(msg, 1); err == nil {
		t.Fatal("send should fail after one retry")
	}
}
