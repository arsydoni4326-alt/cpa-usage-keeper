package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	keeperapi "cpa-usage-keeper/internal/api"
	"cpa-usage-keeper/internal/auth"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/enrichgeo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSessionListIncludesEnrichmentGeoFields verifies that when IP enrichment
// is enabled, the session list response includes loginGeo / lastSeenGeo for
// sessions with valid login/last-seen IPs. Private IPs (e.g. 127.0.0.1) are
// classified locally and marked private; public IPs trigger an async lookup
// and return a pending marker on first read.
func TestSessionListIncludesEnrichmentGeoFields(t *testing.T) {
	db := openEnrichmentTestDatabase(t)
	manager := auth.NewPersistentSessionManager(time.Hour, auth.NewGormSessionStore(db))
	currentToken, _, err := manager.CreateWithSourceAndMetadata(
		auth.SessionSourceStandard,
		auth.SessionClientMetadata{IP: "127.0.0.1"},
	)
	if err != nil {
		t.Fatalf("create current session: %v", err)
	}
	config := keeperapi.AuthConfig{
		Enabled:       true,
		LoginPassword: "test-password",
		SessionTTL:    time.Hour,
	}
	handler := keeperapi.NewAuthHandler(config, manager)
	handler.SetIPEnricher(enrichgeo.NewEnricher(enrichgeo.Options{
		Enabled: true,
		TTL:     time.Hour,
	}, nil))
	router := keeperapi.NewRouter(nil, nil, nil, nil, config, handler, "")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	request.AddCookie(&http.Cookie{Name: standardSessionCookieName, Value: currentToken})
	request.RemoteAddr = "127.0.0.1:42310"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}

	var parsed struct {
		Items []struct {
			ID         string `json:"id"`
			LoginIP    string `json:"loginIp"`
			LastSeenIP string `json:"lastSeenIp"`
			LoginGeo   *struct {
				Enabled bool   `json:"enabled"`
				Private bool   `json:"private"`
				Pending bool   `json:"pending"`
				Hostname string `json:"hostname"`
			} `json:"loginGeo"`
			LastSeenGeo *struct {
				Enabled bool `json:"enabled"`
				Private bool `json:"private"`
			} `json:"lastSeenGeo"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode session list: %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("expected one session, got %d", len(parsed.Items))
	}
	item := parsed.Items[0]
	if item.LoginIP != "127.0.0.1" {
		t.Fatalf("expected LoginIP 127.0.0.1, got %s", item.LoginIP)
	}
	if item.LoginGeo == nil {
		t.Fatal("expected loginGeo to be present when enrichment is enabled")
	}
	if !item.LoginGeo.Enabled || !item.LoginGeo.Private {
		t.Fatalf("expected loginGeo enabled+private for loopback IP, got %+v", item.LoginGeo)
	}
	// lastSeenIP starts empty on creation; geo should be absent when there is no IP.
	if item.LastSeenGeo != nil {
		t.Fatalf("expected lastSeenGeo to be nil for empty IP, got %+v", item.LastSeenGeo)
	}
}

// TestSessionListOmitsGeoWhenDisabled verifies that loginGeo/lastSeenGeo are
// absent when IP enrichment is switched off (the default).
func TestSessionListOmitsGeoWhenDisabled(t *testing.T) {
	db := openEnrichmentTestDatabase(t)
	manager := auth.NewPersistentSessionManager(time.Hour, auth.NewGormSessionStore(db))
	token, _, err := manager.CreateWithSourceAndMetadata(
		auth.SessionSourceStandard,
		auth.SessionClientMetadata{IP: "203.0.113.5"},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	config := keeperapi.AuthConfig{
		Enabled:       true,
		LoginPassword: "test-password",
		SessionTTL:    time.Hour,
	}
	// No enricher installed → feature is disabled.
	router := keeperapi.NewRouter(nil, nil, nil, nil, config, keeperapi.NewAuthHandler(config, manager), "")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	request.AddCookie(&http.Cookie{Name: standardSessionCookieName, Value: token})
	request.RemoteAddr = "127.0.0.1:42310"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "loginGeo") {
		t.Fatalf("expected no loginGeo field when enrichment is disabled, got %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "lastSeenGeo") {
		t.Fatalf("expected no lastSeenGeo field when enrichment is disabled, got %s", response.Body.String())
	}
}

func openEnrichmentTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth-session-enrichment.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&entities.AuthSession{}); err != nil {
		t.Fatalf("auto migrate auth sessions: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
