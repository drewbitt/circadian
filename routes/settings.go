package routes

import (
	"bytes"
	"net/http"

	"github.com/drewbitt/circadian/templates"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func registerSettingsRoutes(se *core.ServeEvent, app *pocketbase.PocketBase) {
	se.Router.GET("/settings", func(re *core.RequestEvent) error {
		info, _ := re.RequestInfo()
		if info.Auth == nil {
			return re.UnauthorizedError("", nil)
		}

		settings, _ := app.FindFirstRecordByFilter("settings", "user = {:user}", map[string]any{"user": info.Auth.Id})
		var buf bytes.Buffer
		templates.Settings(settings).Render(re.Request.Context(), &buf)
		re.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		re.Response.Write(buf.Bytes())
		return nil
	})

	se.Router.POST("/settings", func(re *core.RequestEvent) error {
		info, _ := re.RequestInfo()
		if info.Auth == nil {
			return re.UnauthorizedError("", nil)
		}

		data := struct {
			SleepNeedHours       float64 `json:"sleep_need_hours"`
			NtfyTopic            string  `json:"ntfy_topic"`
			NtfyServer           string  `json:"ntfy_server"`
			NotificationsEnabled bool    `json:"notifications_enabled"`
		}{}
		if err := re.BindBody(&data); err != nil {
			return re.BadRequestError("Invalid data", err)
		}

		// Find or create settings record.
		settings, err := app.FindFirstRecordByFilter("settings", "user = {:user}", map[string]any{"user": info.Auth.Id})
		if err != nil {
			collection, err := app.FindCollectionByNameOrId("settings")
			if err != nil {
				return re.InternalServerError("", err)
			}
			settings = core.NewRecord(collection)
			settings.Set("user", info.Auth.Id)
		}

		if data.SleepNeedHours > 0 {
			settings.Set("sleep_need_hours", data.SleepNeedHours)
		}
		settings.Set("ntfy_topic", data.NtfyTopic)
		if data.NtfyServer != "" {
			settings.Set("ntfy_server", data.NtfyServer)
		}
		settings.Set("notifications_enabled", data.NotificationsEnabled)

		if err := app.Save(settings); err != nil {
			return re.InternalServerError("Failed to save settings", err)
		}

		return re.Redirect(http.StatusSeeOther, "/settings")
	})
}
