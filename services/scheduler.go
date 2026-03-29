package services

import (
	"fmt"
	"log"
	"time"

	"github.com/drewbitt/circadian/engine"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// SchedulerConfig holds notification settings for a user.
type SchedulerConfig struct {
	UserID             string
	NtfyServer         string
	NtfyTopic          string
	SleepNeedHours     float64
	NotificationsEnabled bool
}

// RunMorningJob computes today's energy schedule for a user and dispatches notifications.
// Called by the PocketBase cron scheduler.
func RunMorningJob(app *pocketbase.PocketBase, userID string) error {
	// Load user settings.
	settings, err := app.FindFirstRecordByFilter("settings", "user = {:user}", map[string]any{"user": userID})
	if err != nil {
		return fmt.Errorf("load settings for user %s: %w", userID, err)
	}

	cfg := SchedulerConfig{
		UserID:             userID,
		NtfyServer:         settings.GetString("ntfy_server"),
		NtfyTopic:          settings.GetString("ntfy_topic"),
		SleepNeedHours:     settings.GetFloat("sleep_need_hours"),
		NotificationsEnabled: settings.GetBool("notifications_enabled"),
	}
	if cfg.SleepNeedHours == 0 {
		cfg.SleepNeedHours = 8.0
	}

	// Load sleep records for the past 14 days.
	fourteenDaysAgo := time.Now().AddDate(0, 0, -14).Format("2006-01-02 00:00:00")
	sleepRecords, err := app.FindRecordsByFilter(
		"sleep_records",
		"user = {:user} && date >= {:since}",
		"-date",
		0, 0,
		map[string]any{"user": userID, "since": fourteenDaysAgo},
	)
	if err != nil {
		return fmt.Errorf("load sleep records: %w", err)
	}

	// Convert to engine types.
	var engineRecords []engine.SleepRecord
	var sleepPeriods []engine.SleepPeriod
	for _, r := range sleepRecords {
		engineRecords = append(engineRecords, engine.SleepRecord{
			Date:            r.GetDateTime("date").Time(),
			SleepStart:      r.GetDateTime("sleep_start").Time(),
			SleepEnd:        r.GetDateTime("sleep_end").Time(),
			DurationMinutes: r.GetInt("duration_minutes"),
		})
		sleepPeriods = append(sleepPeriods, engine.SleepPeriod{
			Start: r.GetDateTime("sleep_start").Time(),
			End:   r.GetDateTime("sleep_end").Time(),
		})
	}

	// Calculate sleep debt.
	debt := engine.CalculateSleepDebt(engineRecords, cfg.SleepNeedHours, time.Now())

	// Find most recent wake time (end of last sleep).
	wakeTime := time.Now().Truncate(24 * time.Hour).Add(7 * time.Hour) // default 7am
	if len(sleepPeriods) > 0 {
		latest := sleepPeriods[0]
		for _, sp := range sleepPeriods {
			if sp.End.After(latest.End) {
				latest = sp
			}
		}
		wakeTime = latest.End
	}

	// Predict energy for the next 24 hours.
	predStart := wakeTime
	predEnd := wakeTime.Add(24 * time.Hour)
	points := engine.PredictEnergy(sleepPeriods, predStart, predEnd)

	// Classify zones.
	schedule := engine.ClassifyZones(points, wakeTime)

	// Store the energy schedule.
	if err := storeSchedule(app, userID, schedule); err != nil {
		log.Printf("Failed to store schedule for user %s: %v", userID, err)
	}

	// Send morning notification.
	if cfg.NotificationsEnabled && cfg.NtfyTopic != "" {
		morningMsg := fmt.Sprintf(
			"Sleep debt: %.1fh (%s). Best focus: %s-%s.",
			debt.Hours, debt.Category,
			schedule.BestFocusStart.Format("3:04pm"),
			schedule.BestFocusEnd.Format("3:04pm"),
		)
		if err := SendNotification(cfg.NtfyServer, cfg.NtfyTopic, "Good morning!", morningMsg, 3); err != nil {
			log.Printf("Failed morning notification for user %s: %v", userID, err)
		}

		// Schedule intra-day notifications.
		scheduleIntraDayNotifications(cfg, schedule)
	}

	return nil
}

func storeSchedule(app *pocketbase.PocketBase, userID string, schedule engine.Schedule) error {
	collection, err := app.FindCollectionByNameOrId("energy_schedules")
	if err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")

	// Try to find existing schedule for today.
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

	record.Set("wake_time", schedule.WakeTime)
	record.Set("schedule_json", schedule.Points)

	return app.Save(record)
}

func scheduleIntraDayNotifications(cfg SchedulerConfig, schedule engine.Schedule) {
	now := time.Now()

	type notification struct {
		at       time.Time
		title    string
		message  string
		priority int
	}

	notifications := []notification{
		{
			at:       schedule.CaffeineCutoff.Add(-30 * time.Minute),
			title:    "Caffeine Cutoff Soon",
			message:  fmt.Sprintf("Last call for caffeine at %s", schedule.CaffeineCutoff.Format("3:04pm")),
			priority: 3,
		},
		{
			at:       schedule.MelatoninWindow.Add(-30 * time.Minute),
			title:    "Melatonin Window Opening",
			message:  "Your melatonin window opens in 30 minutes. Start winding down.",
			priority: 4,
		},
	}

	// Add nap window notification if applicable.
	if !schedule.OptimalNapStart.IsZero() {
		notifications = append(notifications, notification{
			at:       schedule.OptimalNapStart,
			title:    "Optimal Nap Window",
			message:  fmt.Sprintf("Good time for a 20-min nap until %s", schedule.OptimalNapEnd.Format("3:04pm")),
			priority: 2,
		})
	}

	for _, n := range notifications {
		if n.at.After(now) {
			delay := n.at.Sub(now)
			go func(n notification) {
				time.Sleep(delay)
				if err := SendNotification(cfg.NtfyServer, cfg.NtfyTopic, n.title, n.message, n.priority); err != nil {
					log.Printf("Failed notification %q: %v", n.title, err)
				}
			}(n)
		}
	}
}
