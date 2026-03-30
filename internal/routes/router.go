// Package routes registers all HTTP route groups on PocketBase.
package routes

import (
	"errors"
	"net/url"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// userLocationFromForm reads the IANA timezone name from a hidden "tz" form
// field first, then falls back to the cookie. Preferred for form submissions
// where the field is always present, avoiding stale/missing cookie issues.
func userLocationFromForm(re *core.RequestEvent) *time.Location {
	if tz := re.Request.FormValue("tz"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return userLocation(re)
}

// userLocation reads the IANA timezone name from the browser-set "tz" cookie
// and returns the corresponding *time.Location. Falls back to time.UTC if the
// cookie is absent or names an unrecognised timezone.
func userLocation(re *core.RequestEvent) *time.Location {
	cookie, err := re.Request.Cookie("tz")
	if err != nil {
		return time.UTC
	}
	name, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

var errNotAuthenticated = errors.New("not authenticated")

// authedUserID extracts the authenticated user's ID from the request,
// returning errNotAuthenticated if the request has no valid session.
func authedUserID(re *core.RequestEvent) (string, error) {
	info, err := re.RequestInfo()
	if err != nil {
		return "", err
	}
	if info.Auth == nil {
		return "", errNotAuthenticated
	}
	return info.Auth.Id, nil
}

// Register binds all application routes to the PocketBase router.
func Register(app *pocketbase.PocketBase) {
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id: "meridian-routes",
		Func: func(se *core.ServeEvent) error {
			registerAuthRoutes(se, app)
			registerDashboardRoutes(se, app)
			registerSleepRoutes(se, app)
			registerSettingsRoutes(se, app)
			registerFitbitAuthRoutes(se, app)
			registerAPIRoutes(se, app)
			return se.Next()
		},
	})
}
