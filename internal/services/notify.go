package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errNtfyStatus = errors.New("ntfy returned error status")

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Action represents a notification action button.
type Action struct {
	Type  string `json:"action"`
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

// Notification represents a push notification via ntfy.
type Notification struct {
	Server      string
	Topic       string
	AccessToken string
	Title       string
	Message     string
	Priority    int
	At          time.Time // scheduled delivery time (zero = immediate)
	Tags        []string
	Click       string
	Actions     []Action
}

// SendNotification sends a push notification via ntfy.
func SendNotification(n Notification) error {
	if n.Server == "" {
		n.Server = "https://ntfy.sh"
	}
	url := strings.TrimRight(n.Server, "/") + "/" + n.Topic

	req, err := http.NewRequest("POST", url, strings.NewReader(n.Message))
	if err != nil {
		return fmt.Errorf("create ntfy request: %w", err)
	}

	req.Header.Set("Title", n.Title)
	req.Header.Set("Priority", strconv.Itoa(n.Priority))

	if !n.At.IsZero() {
		if n.At.Before(time.Now()) {
			return nil // past, skip silently
		}
		req.Header.Set("At", strconv.FormatInt(n.At.Unix(), 10))
	}
	if len(n.Tags) > 0 {
		req.Header.Set("Tags", strings.Join(n.Tags, ","))
	}
	if n.Click != "" {
		req.Header.Set("Click", n.Click)
	}
	if n.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.AccessToken)
	}
	if len(n.Actions) > 0 {
		b, err := json.Marshal(n.Actions)
		if err != nil {
			return fmt.Errorf("marshal ntfy actions: %w", err)
		}
		req.Header.Set("Actions", string(b))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned status %d: %w", resp.StatusCode, errNtfyStatus)
	}
	return nil
}
