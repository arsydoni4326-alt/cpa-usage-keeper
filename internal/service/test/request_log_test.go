package test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/cpa"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	"cpa-usage-keeper/internal/service"
	"gorm.io/gorm"
)

type requestLogClientStub struct {
	calls  int
	result *cpa.RequestLogResult
	err    error
}

func (s *requestLogClientStub) FetchRequestLogByID(context.Context, string) (*cpa.RequestLogResult, error) {
	s.calls++
	return s.result, s.err
}

func TestRequestLogServiceLoadsEventLogAndCachesByRequestID(t *testing.T) {
	db := openRequestLogTestDB(t)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:    "event-1",
		RequestID:   "req-log-1",
		Timestamp:   time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local),
		Model:       "claude-sonnet",
		Source:      "source",
		AuthIndex:   "auth-1",
		APIGroupKey: "group",
	}}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}

	client := &requestLogClientStub{result: &cpa.RequestLogResult{
		StatusCode: http.StatusOK,
		Filename:   "v1-responses-req-log-1.log",
		Body:       []byte("=== REQUEST INFO ===\nURL: /v1/responses\n=== API RESPONSE ===\n{\"ok\":true}\n"),
	}}
	provider := service.NewRequestLogService(db, client)

	first, err := provider.GetUsageEventRequestLog(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetUsageEventRequestLog returned error: %v", err)
	}
	if first.RequestID != "req-log-1" || first.Filename != "v1-responses-req-log-1.log" || first.Cached {
		t.Fatalf("unexpected first response: %+v", first)
	}
	if len(first.Sections) != 2 || first.Sections[0].Title != "REQUEST INFO" || first.Sections[1].Title != "API RESPONSE" {
		t.Fatalf("unexpected sections: %+v", first.Sections)
	}

	second, err := provider.GetUsageEventRequestLog(context.Background(), 1)
	if err != nil {
		t.Fatalf("cached GetUsageEventRequestLog returned error: %v", err)
	}
	if !second.Cached {
		t.Fatalf("expected cached response, got %+v", second)
	}
	if client.calls != 1 {
		t.Fatalf("expected one CPA call, got %d", client.calls)
	}
}

func TestRequestLogServiceMapsCPANotFoundToUnavailable(t *testing.T) {
	db := openRequestLogTestDB(t)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:  "event-404",
		RequestID: "req-missing",
	}}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}

	client := &requestLogClientStub{
		result: &cpa.RequestLogResult{StatusCode: http.StatusNotFound, Body: []byte(`{"error":"missing"}`)},
		err:    errors.New("management request log request returned status 404"),
	}
	provider := service.NewRequestLogService(db, client)

	_, err := provider.GetUsageEventRequestLog(context.Background(), 1)
	if !errors.Is(err, service.ErrRequestLogUnavailable) {
		t.Fatalf("expected ErrRequestLogUnavailable, got %v", err)
	}
}

func TestRequestLogServicePrunesCacheEntries(t *testing.T) {
	db := openRequestLogTestDB(t)
	events := make([]entities.UsageEvent, 0, 140)
	for i := 0; i < 140; i++ {
		events = append(events, entities.UsageEvent{
			EventKey:  "event-prune-" + strconv.Itoa(i),
			RequestID: "req-prune-" + strconv.Itoa(i),
		})
	}
	if _, _, err := repository.InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	client := &requestLogClientStub{result: &cpa.RequestLogResult{
		StatusCode: http.StatusOK,
		Filename:   "request.log",
		Body:       []byte("=== REQUEST INFO ===\nURL: /v1/responses\n"),
	}}
	provider := service.NewRequestLogService(db, client)

	for eventID := int64(1); eventID <= int64(len(events)); eventID++ {
		if _, err := provider.GetUsageEventRequestLog(context.Background(), eventID); err != nil {
			t.Fatalf("load request log for event %d: %v", eventID, err)
		}
	}
	if client.calls != len(events) {
		t.Fatalf("expected initial CPA calls to match event count, got %d", client.calls)
	}

	if _, err := provider.GetUsageEventRequestLog(context.Background(), 1); err != nil {
		t.Fatalf("reload pruned request log: %v", err)
	}
	if client.calls != len(events)+1 {
		t.Fatalf("expected oldest cache entry to be pruned and refetched, got %d calls", client.calls)
	}
}

func openRequestLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "request-log.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}
