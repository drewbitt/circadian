// Package routes registers all HTTP route groups on PocketBase.
package routes

import (
	"errors"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

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
