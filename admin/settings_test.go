package admin

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieptran/copilot-proxy/db"
)

func TestSettingsPageRendersCurrentValue(t *testing.T) {
	adminHandler, cleanup := newTestAdmin(t)
	defer cleanup()

	mux := http.NewServeMux()
	adminHandler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	addAuthenticatedSession(t, adminHandler, req)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "value=\"20\"") {
		t.Fatalf("body = %q, want current cadence", rec.Body.String())
	}
}

func TestSettingsUpdateChangesRuntimeValue(t *testing.T) {
	adminHandler, cleanup := newTestAdmin(t)
	defer cleanup()

	mux := http.NewServeMux()
	adminHandler.RegisterRoutes(mux)

	form := url.Values{}
	form.Set("user_every", "7")

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuthenticatedSession(t, adminHandler, req)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := adminHandler.initiatorSettings.GetUserEvery(); got != 7 {
		t.Fatalf("runtime value = %d, want 7", got)
	}
}

func TestSettingsUpdateRejectsInvalidValue(t *testing.T) {
	adminHandler, cleanup := newTestAdmin(t)
	defer cleanup()

	mux := http.NewServeMux()
	adminHandler.RegisterRoutes(mux)

	form := url.Values{}
	form.Set("user_every", "0")

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuthenticatedSession(t, adminHandler, req)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := adminHandler.initiatorSettings.GetUserEvery(); got != 20 {
		t.Fatalf("runtime value = %d, want unchanged 20", got)
	}
}

func newTestAdmin(t *testing.T) (*Admin, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	database, err := db.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}

	tmpls := map[string]*template.Template{
		"login":    template.Must(template.New("login").Parse(`login`)),
		"users":    template.Must(template.New("layout").Parse(`{{define "layout"}}users{{end}}`)),
		"report":   template.Must(template.New("layout").Parse(`{{define "layout"}}report{{end}}`)),
		"settings": template.Must(template.New("layout").Parse(`{{define "layout"}}<input value="{{.UserEvery}}">{{end}}`)),
	}

	settings := &testInitiatorSettings{userEvery: 20}
	adminHandler := New(database, tmpls, settings)

	cleanup := func() {
		_ = database.Close()
		_ = os.Remove(filepath.Join(tmpDir, "test.db"))
	}

	return adminHandler, cleanup
}

func addAuthenticatedSession(t *testing.T, adminHandler *Admin, req *http.Request) {
	t.Helper()

	sessionToken, csrfToken, err := adminHandler.sessions.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession(): %v", err)
	}

	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken, Path: "/admin"})
	req.Header.Set("X-CSRF-Token", csrfToken)
}

type testInitiatorSettings struct {
	userEvery int
}

func (t *testInitiatorSettings) GetUserEvery() int {
	return t.userEvery
}

func (t *testInitiatorSettings) SetUserEvery(userEvery int) {
	t.userEvery = userEvery
}
