package push

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		if r.URL.Path != "/api/v1/push" {
			t.Errorf("path = %s, want /api/v1/push", r.URL.Path)
		}

		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", got)
		}

		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			return
		}

		if body.RequestID == "" {
			t.Error("request_id is empty")
		}
		if body.RecipientID != "driver-123" {
			t.Errorf("recipient_id = %s, want driver-123", body.RecipientID)
		}
		if body.Kind != "REQUEST_POSITION" {
			t.Errorf("kind = %s, want REQUEST_POSITION", body.Kind)
		}
		if body.Data["trip_id"] != "trip-123" {
			t.Errorf("data.trip_id = %s, want trip-123", body.Data["trip_id"])
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	defer client.Close()

	if err := client.Send(context.Background(), "trip-123", "driver-123"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestClientSendHTTPError(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusInternalServerError,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			client := NewClient(server.URL, time.Second)
			defer client.Close()

			err := client.Send(context.Background(), "trip-123", "driver-123")
			if err == nil {
				t.Fatal("Send() error = nil, want HTTP error")
			}
			if !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Errorf("error %q does not contain status %d", err, status)
			}
		})
	}
}

func TestClientSendTransportError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", time.Second)
	defer client.Close()

	err := client.Send(context.Background(), "trip-123", "driver-123")
	if err == nil {
		t.Fatal("Send() error = nil, want transport error")
	}
	if !strings.Contains(err.Error(), "push transport error") {
		t.Errorf("unexpected error: %v", err)
	}
}
