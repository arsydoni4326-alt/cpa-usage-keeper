package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cpa-usage-keeper/internal/cpa"
	"cpa-usage-keeper/internal/repository"
	"gorm.io/gorm"
)

const (
	requestLogCacheTTL         = 10 * time.Minute
	requestLogNegativeCacheTTL = time.Minute
	requestLogMaxBytes         = 5 * 1024 * 1024
	requestLogCacheMaxEntries  = 128
)

var (
	ErrRequestLogUnavailable = errors.New("request log unavailable")
	ErrRequestLogMissingID   = errors.New("usage event request id missing")
	ErrRequestLogTooLarge    = errors.New("request log too large")
)

type RequestLogClient interface {
	FetchRequestLogByID(ctx context.Context, requestID string) (*cpa.RequestLogResult, error)
}

type RequestLogProvider interface {
	GetUsageEventRequestLog(ctx context.Context, eventID int64) (RequestLogResponse, error)
}

type RequestLogResponse struct {
	EventID   int64
	RequestID string
	Filename  string
	Cached    bool
	Available bool
	Sections  []RequestLogSection
	Raw       string
}

type RequestLogSection struct {
	Title   string
	Content string
}

type requestLogService struct {
	db     *gorm.DB
	client RequestLogClient
	now    func() time.Time

	mu            sync.Mutex
	cache         map[string]requestLogCacheEntry
	cacheSequence int64
}

type requestLogCacheEntry struct {
	response  RequestLogResponse
	err       error
	expiresAt time.Time
	createdAt time.Time
	sequence  int64
}

func NewRequestLogService(db *gorm.DB, client RequestLogClient) RequestLogProvider {
	return &requestLogService{
		db:     db,
		client: client,
		now:    time.Now,
		cache:  map[string]requestLogCacheEntry{},
	}
}

func (s *requestLogService) GetUsageEventRequestLog(ctx context.Context, eventID int64) (RequestLogResponse, error) {
	if s == nil {
		return RequestLogResponse{}, fmt.Errorf("request log service is nil")
	}
	if s.db == nil {
		return RequestLogResponse{}, fmt.Errorf("database is nil")
	}
	if s.client == nil {
		return RequestLogResponse{}, fmt.Errorf("request log client is not configured")
	}
	requestID, err := repository.FindUsageEventRequestIDByID(s.db.WithContext(ctx), eventID)
	if err != nil {
		return RequestLogResponse{}, err
	}
	if requestID == "" {
		return RequestLogResponse{EventID: eventID, Available: false}, ErrRequestLogMissingID
	}

	if cached, ok := s.getCached(requestID); ok {
		cached.response.EventID = eventID
		cached.response.Cached = true
		return cached.response, cached.err
	}

	result, err := s.client.FetchRequestLogByID(ctx, requestID)
	if err != nil {
		if result != nil && result.StatusCode == http.StatusNotFound {
			response := RequestLogResponse{EventID: eventID, RequestID: requestID, Available: false}
			s.setCached(requestID, response, ErrRequestLogUnavailable, requestLogNegativeCacheTTL)
			return response, ErrRequestLogUnavailable
		}
		return RequestLogResponse{}, err
	}
	if result == nil {
		return RequestLogResponse{}, fmt.Errorf("request log result is nil")
	}
	if len(result.Body) > requestLogMaxBytes {
		return RequestLogResponse{EventID: eventID, RequestID: requestID, Filename: result.Filename, Available: false}, ErrRequestLogTooLarge
	}

	raw := string(result.Body)
	response := RequestLogResponse{
		EventID:   eventID,
		RequestID: requestID,
		Filename:  strings.TrimSpace(result.Filename),
		Available: true,
		Sections:  ParseRequestLogSections(raw),
		Raw:       raw,
	}
	s.setCached(requestID, response, nil, requestLogCacheTTL)
	return response, nil
}

func (s *requestLogService) getCached(requestID string) (requestLogCacheEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[requestID]
	if !ok {
		return requestLogCacheEntry{}, false
	}
	if !s.now().Before(entry.expiresAt) {
		delete(s.cache, requestID)
		return requestLogCacheEntry{}, false
	}
	return entry, true
}

func (s *requestLogService) setCached(requestID string, response RequestLogResponse, err error, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.cacheSequence++
	s.cache[requestID] = requestLogCacheEntry{
		response:  response,
		err:       err,
		expiresAt: now.Add(ttl),
		createdAt: now,
		sequence:  s.cacheSequence,
	}
	s.pruneCacheLocked(now)
}

func (s *requestLogService) pruneCacheLocked(now time.Time) {
	for requestID, entry := range s.cache {
		if !now.Before(entry.expiresAt) {
			delete(s.cache, requestID)
		}
	}
	for len(s.cache) > requestLogCacheMaxEntries {
		oldestRequestID := ""
		var oldestSequence int64
		for requestID, entry := range s.cache {
			if oldestRequestID == "" || entry.sequence < oldestSequence {
				oldestRequestID = requestID
				oldestSequence = entry.sequence
			}
		}
		if oldestRequestID == "" {
			return
		}
		delete(s.cache, oldestRequestID)
	}
}

func ParseRequestLogSections(raw string) []RequestLogSection {
	lines := strings.Split(raw, "\n")
	sections := make([]RequestLogSection, 0, 8)
	currentTitle := ""
	currentLines := []string{}
	flush := func() {
		if currentTitle == "" {
			return
		}
		sections = append(sections, RequestLogSection{
			Title:   currentTitle,
			Content: strings.TrimRight(strings.Join(currentLines, "\n"), "\n"),
		})
		currentLines = []string{}
	}
	for _, line := range lines {
		title, ok := parseRequestLogSectionTitle(line)
		if ok {
			flush()
			currentTitle = title
			continue
		}
		if currentTitle != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()
	if len(sections) == 0 && strings.TrimSpace(raw) != "" {
		return []RequestLogSection{{Title: "RAW LOG", Content: raw}}
	}
	return sections
}

func parseRequestLogSectionTitle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "===") || !strings.HasSuffix(trimmed, "===") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "==="), "==="))
	if title == "" {
		return "", false
	}
	return title, true
}
