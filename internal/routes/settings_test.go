package routes

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/drewbitt/circadian/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

const testUserEmail = "test@example.com"

func setupApp(t testing.TB) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.EnsureCollections(app); err != nil {
		t.Fatal(err)
	}
	return app
}

func tokenFor(t testing.TB, app *tests.TestApp, email string) string {
	t.Helper()
	user, err := app.FindAuthRecordByEmail("users", email)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func hcMultipart(t testing.TB) (*bytes.Buffer, string) {
	t.Helper()
	json := `{"sleepSessions":[{
		"startTime":"2024-01-15T23:00:00Z","endTime":"2024-01-16T07:00:00Z",
		"stages":[
			{"startTime":"2024-01-15T23:00:00Z","endTime":"2024-01-16T01:00:00Z","stage":4},
			{"startTime":"2024-01-16T01:00:00Z","endTime":"2024-01-16T03:00:00Z","stage":5},
			{"startTime":"2024-01-16T03:00:00Z","endTime":"2024-01-16T05:00:00Z","stage":6},
			{"startTime":"2024-01-16T05:00:00Z","endTime":"2024-01-16T07:00:00Z","stage":4}
		]
	}]}`

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormField("source")
	fmt.Fprint(fw, "healthconnect")
	fw, _ = w.CreateFormFile("file", "export.json")
	fmt.Fprint(fw, json)
	w.Close()
	return &buf, w.FormDataContentType()
}

func expectLocation(t testing.TB, res *http.Response, substr string) {
	t.Helper()
	loc := res.Header.Get("Location")
	if !strings.Contains(loc, substr) {
		t.Errorf("Location header %q does not contain %q", loc, substr)
	}
}

// TestSettingsImport_Redirect would have FAILED before the fix: the form posted
// to /api/import which returned 200+JSON. Now /settings/import returns 303.
func TestSettingsImport_Redirect(t *testing.T) {
	body, ct := hcMultipart(t)
	headers := map[string]string{"Content-Type": ct}

	(&tests.ApiScenario{
		Name:           "import redirects with count",
		Method:         http.MethodPost,
		URL:            "/settings/import",
		Body:           body,
		ExpectedStatus: 303,
		TestAppFactory: func(t testing.TB) *tests.TestApp { return setupApp(t) },
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			registerSettingsRoutes(e, app)
			headers["Authorization"] = tokenFor(t, app, testUserEmail)
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			expectLocation(t, res, "/settings?imported=1")
		},
		Headers: headers,
	}).Test(t)
}

func TestSettingsImport_NoFile(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormField("source")
	fmt.Fprint(fw, "healthconnect")
	w.Close()

	headers := map[string]string{"Content-Type": w.FormDataContentType()}

	(&tests.ApiScenario{
		Name:           "no file redirects with error",
		Method:         http.MethodPost,
		URL:            "/settings/import",
		Body:           &buf,
		ExpectedStatus: 303,
		TestAppFactory: func(t testing.TB) *tests.TestApp { return setupApp(t) },
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			registerSettingsRoutes(e, app)
			headers["Authorization"] = tokenFor(t, app, testUserEmail)
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			expectLocation(t, res, "/settings?import_error=")
		},
		Headers: headers,
	}).Test(t)
}

func TestSettingsImport_Unauthenticated(t *testing.T) {
	body, ct := hcMultipart(t)

	(&tests.ApiScenario{
		Name:           "unauthenticated redirects to login",
		Method:         http.MethodPost,
		URL:            "/settings/import",
		Body:           body,
		ExpectedStatus: 307,
		TestAppFactory: func(t testing.TB) *tests.TestApp { return setupApp(t) },
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			registerSettingsRoutes(e, app)
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			expectLocation(t, res, "/login")
		},
		Headers: map[string]string{"Content-Type": ct},
	}).Test(t)
}

// TestAPIImport_ReturnsJSON confirms the JSON API still returns JSON.
func TestAPIImport_ReturnsJSON(t *testing.T) {
	body, ct := hcMultipart(t)
	headers := map[string]string{"Content-Type": ct}

	(&tests.ApiScenario{
		Name:            "api import returns JSON not redirect",
		Method:          http.MethodPost,
		URL:             "/api/import?source=healthconnect",
		Body:            body,
		ExpectedStatus:  200,
		ExpectedContent: []string{`"imported"`, `"total"`},
		TestAppFactory:  func(t testing.TB) *tests.TestApp { return setupApp(t) },
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			registerAPIRoutes(e, app)
			headers["Authorization"] = tokenFor(t, app, testUserEmail)
		},
		Headers: headers,
	}).Test(t)
}
