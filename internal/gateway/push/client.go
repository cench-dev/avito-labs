package push

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Client struct {
	url        string
	httpClient *http.Client
}

type request struct {
	RequestID   string            `json:"request_id"`
	RecipientID string            `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data"`
}

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		url: strings.TrimRight(url, "/") + "/api/v1/push",
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Send(ctx context.Context, tripID, driverID string) error {
	tracer := otel.Tracer("trip-service/push")
	ctx, span := tracer.Start(ctx, "push.send")
	defer span.End()

	span.SetAttributes(
		attribute.String("trip.id", tripID),
		attribute.String("driver.id", driverID),
	)

	requestID, err := newRequestID()
	if err != nil {
		err = fmt.Errorf("generate push request id: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	body, err := json.Marshal(request{
		RequestID:   requestID,
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data: map[string]string{
			"trip_id": tripID,
		},
	})
	if err != nil {
		err = fmt.Errorf("marshal push request: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.url,
		bytes.NewReader(body),
	)
	if err != nil {
		err = fmt.Errorf("create push request: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("push transport error: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusAccepted {
		err = fmt.Errorf("push service returned HTTP status %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func newRequestID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(id[:]), nil
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
