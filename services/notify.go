// Package services provides notification dispatch and scheduling.
package services

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// SendNotification sends a push notification via ntfy.
func SendNotification(server, topic, title, message string, priority int) error {
	if server == "" {
		server = "https://ntfy.sh"
	}
	url := strings.TrimRight(server, "/") + "/" + topic

	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("create ntfy request: %w", err)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", strconv.Itoa(priority))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}
	return nil
}
