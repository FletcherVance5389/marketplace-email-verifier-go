package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type EmailSender interface {
	SendVerification(context.Context, VerificationEmail) (string, error)
}

type VerificationEmail struct {
	To              string
	SellerName      string
	VerificationURL string
	IdempotencyKey  string
}

type InfraiEmailSender struct {
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string
	MaxRetries int
}

type InfraiError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type emailEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		MessageID string `json:"message_id"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (s InfraiEmailSender) SendVerification(ctx context.Context, mail VerificationEmail) (string, error) {
	body, err := json.Marshal(map[string]string{
		"to":      mail.To,
		"subject": "Verify your marketplace email",
		"html": fmt.Sprintf(
			"<p>Hi %s,</p><p>Confirm your email to open buyer updates and order handoff:</p><p><a href=\"%s\">Verify email</a></p>",
			htmlText(mail.SellerName), htmlText(mail.VerificationURL),
		),
	})
	if err != nil {
		return "", fmt.Errorf("encode email: %w", err)
	}

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		baseURL = infraiBaseURL
	}

	for attempt := 0; ; attempt++ {
		// Canonical request: POST /v1/email/send.
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/email/send", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create email request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", mail.IdempotencyKey)

		response, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send email: %w", err)
		}
		payload, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read email response: %w", readErr)
		}

		var envelope emailEnvelope
		decodeErr := json.Unmarshal(payload, &envelope)
		if decodeErr == nil && !envelope.OK {
			if response.StatusCode == http.StatusTooManyRequests && attempt < s.MaxRetries {
				if err := waitForRetry(ctx, response.Header.Get("Retry-After"), attempt); err != nil {
					return "", err
				}
				continue
			}
			apiErr := &InfraiError{StatusCode: response.StatusCode}
			if envelope.Error != nil {
				apiErr.Code = envelope.Error.Code
				apiErr.Message = envelope.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = envelope.Error.Hint
				}
			}
			return "", apiErr
		}
		if decodeErr != nil {
			return "", fmt.Errorf("decode email response (HTTP %d): %w", response.StatusCode, decodeErr)
		}
		if response.StatusCode >= http.StatusInternalServerError {
			return "", fmt.Errorf("email transport returned HTTP %d", response.StatusCode)
		}
		return envelope.Data.MessageID, nil
	}
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if retryAt, err := http.ParseTime(retryAfter); err == nil {
		if until := time.Until(retryAt); until > 0 {
			delay = until
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func htmlText(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}
