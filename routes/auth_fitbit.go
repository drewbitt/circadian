package routes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/drewbitt/circadian/ingest"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/oauth2"
)

func registerFitbitAuthRoutes(se *core.ServeEvent, app *pocketbase.PocketBase) {
	// Initiate Fitbit OAuth flow.
	se.Router.GET("/auth/fitbit", func(re *core.RequestEvent) error {
		info, _ := re.RequestInfo()
		if info.Auth == nil {
			return re.UnauthorizedError("", nil)
		}

		state := generateState()

		// Store state in a temporary way (could use session/cookie in production).
		// For simplicity, we encode user ID in the state.
		stateValue := info.Auth.Id + ":" + state

		cfg := fitbitConfig(re.Request)
		url := cfg.AuthCodeURL(stateValue, oauth2.AccessTypeOffline)
		return re.Redirect(http.StatusTemporaryRedirect, url)
	})

	// OAuth callback.
	se.Router.GET("/auth/fitbit/callback", func(re *core.RequestEvent) error {
		code := re.Request.URL.Query().Get("code")
		state := re.Request.URL.Query().Get("state")

		if code == "" || state == "" {
			return re.BadRequestError("Missing code or state", nil)
		}

		// Extract user ID from state.
		userID := ""
		for i, c := range state {
			if c == ':' {
				userID = state[:i]
				break
			}
		}
		if userID == "" {
			return re.BadRequestError("Invalid state", nil)
		}

		cfg := fitbitConfig(re.Request)
		token, err := cfg.Exchange(context.Background(), code)
		if err != nil {
			return re.InternalServerError("Token exchange failed", err)
		}

		// Store tokens in user settings.
		settings, err := app.FindFirstRecordByFilter("settings", "user = {:user}", map[string]any{"user": userID})
		if err != nil {
			collection, _ := app.FindCollectionByNameOrId("settings")
			settings = core.NewRecord(collection)
			settings.Set("user", userID)
		}

		settings.Set("fitbit_access_token", token.AccessToken)
		settings.Set("fitbit_refresh_token", token.RefreshToken)
		settings.Set("fitbit_token_expiry", token.Expiry)

		if err := app.Save(settings); err != nil {
			return re.InternalServerError("Failed to save tokens", err)
		}

		return re.Redirect(http.StatusSeeOther, "/settings?fitbit=connected")
	})
}

func fitbitConfig(r *http.Request) *oauth2.Config {
	cfg := *ingest.FitbitOAuthConfig
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	cfg.RedirectURL = scheme + "://" + r.Host + "/auth/fitbit/callback"
	return &cfg
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
