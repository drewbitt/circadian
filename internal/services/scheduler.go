package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/drewbitt/meridian/internal/engine"
	"github.com/pocketbase/pocketbase/core"
)

// UpdateUserSchedule computes and stores the energy schedule for a user.
// It does not dispatch notifications — call RunMorningJob for that.
func UpdateUserSchedule(app core.App, userID string) error {
	schedule, rawPoints, _, err := ComputeUserSchedule(app, userID)
	if err != nil {
		return fmt.Errorf("compute schedule: %w", err)
	}
	return storeSchedule(app, userID, schedule.WakeTime, rawPoints)
}

// RunMorningJob computes and stores the energy schedule for a user,
// and dispatches scheduled notifications if enabled.
// It is idempotent per day — returns early if a schedule already exists for today.
func RunMorningJob(app core.App, userID string) error {
	loc := UserLocation(app, userID)
	today := time.Now().In(loc).Format("2006-01-02")

	// Dedupe: only run once per user per day (prevents duplicate notifications
	// when the cron fires multiple times within the morning hour).
	existing, _ := app.FindFirstRecordByFilter("energy_schedules",
		"user = {:user} && date = {:date}",
		map[string]any{"user": userID, "date": today},
	)
	if existing != nil {
		return nil
	}

	settings, err := app.FindFirstRecordByFilter("settings", "user = {:user}", map[string]any{"user": userID})
	if err != nil {
		return fmt.Errorf("load settings for user %s: %w", userID, err)
	}

	schedule, rawPoints, debt, err := ComputeUserSchedule(app, userID)
	if err != nil {
		return fmt.Errorf("compute schedule: %w", err)
	}

	if err := storeSchedule(app, userID, schedule.WakeTime, rawPoints); err != nil {
		slog.Error("failed to store schedule", "user_id", userID, "error", err)
	}

	if settings.GetBool("notifications_enabled") && settings.GetString("ntfy_topic") != "" {
		siteURL := settings.GetString("site_url")
		morningMsg := fmt.Sprintf(
			"Sleep debt: %.1fh (%s). Best focus: %s-%s.",
			debt.Hours, debt.Category,
			schedule.BestFocusStart.In(loc).Format("3:04pm"),
			schedule.BestFocusEnd.In(loc).Format("3:04pm"),
		)
		if err := SendNotification(buildNotif(settings, siteURL,
			"Good morning!",
			morningMsg,
			3,
			time.Time{},
			[]string{"sunny", "battery"},
		)); err != nil {
			slog.Error("failed morning notification", "user_id", userID, "error", err)
		}

		dispatchScheduledNotifications(settings, siteURL, schedule, loc)
	}

	return nil
}

func storeSchedule(app core.App, userID string, wakeTime time.Time, rawPoints []engine.EnergyPoint) error {
	collection, err := app.FindCollectionByNameOrId("energy_schedules")
	if err != nil {
		return err
	}

	today := time.Now().In(wakeTime.Location()).Format("2006-01-02")

	existing, err := app.FindFirstRecordByFilter("energy_schedules",
		"user = {:user} && date = {:date}",
		map[string]any{"user": userID, "date": today},
	)

	var record *core.Record
	if err == nil && existing != nil {
		record = existing
	} else {
		record = core.NewRecord(collection)
		record.Set("user", userID)
		record.Set("date", today)
	}

	record.Set("wake_time", wakeTime)
	record.Set("schedule_json", rawPoints)

	return app.Save(record)
}

// buildNotif constructs a Notification from settings fields.
func buildNotif(settings *core.Record, siteURL, title, message string, priority int, at time.Time, tags []string) Notification {
	return Notification{
		Server:      settings.GetString("ntfy_server"),
		Topic:       settings.GetString("ntfy_topic"),
		AccessToken: settings.GetString("ntfy_access_token"),
		Title:       title,
		Message:     message,
		Priority:    priority,
		At:          at,
		Tags:        tags,
		Click:       dashboardURL(siteURL),
		Actions:     dashboardAction(siteURL),
	}
}

func dispatchScheduledNotifications(settings *core.Record, siteURL string, schedule engine.Schedule, loc *time.Location) {
	notifs := []Notification{
		buildNotif(settings, siteURL,
			"Caffeine Cutoff Soon",
			fmt.Sprintf("Last call for caffeine at %s", schedule.CaffeineCutoff.In(loc).Format("3:04pm")),
			3,
			schedule.CaffeineCutoff.Add(-30*time.Minute),
			[]string{"coffee", "warning"},
		),
		buildNotif(settings, siteURL,
			"Melatonin Window Opening",
			"Your melatonin window opens in 30 minutes. Start winding down.",
			4,
			schedule.MelatoninWindow.Add(-30*time.Minute),
			[]string{"crescent_moon", "zzz"},
		),
	}

	if !schedule.OptimalNapStart.IsZero() {
		notifs = append(notifs, buildNotif(settings, siteURL,
			"Optimal Nap Window",
			fmt.Sprintf("Good time for a 20-min nap until %s", schedule.OptimalNapEnd.In(loc).Format("3:04pm")),
			2,
			schedule.OptimalNapStart,
			[]string{"bed", "battery"},
		))
	}

	for _, n := range notifs {
		if err := SendNotification(n); err != nil {
			slog.Error("failed scheduled notification", "title", n.Title, "error", err)
		}
	}
}

func dashboardURL(siteURL string) string {
	if siteURL == "" {
		return ""
	}
	return strings.TrimRight(siteURL, "/") + "/"
}

func dashboardAction(siteURL string) []Action {
	url := dashboardURL(siteURL)
	if url == "" {
		return nil
	}
	return []Action{{Type: "view", Label: "Dashboard", URL: url}}
}
