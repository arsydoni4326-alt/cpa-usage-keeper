package service

import (
	"context"

	"cpa-usage-keeper/internal/repository/dto"
)

type UsageProvider interface {
	GetUsageWithFilter(context.Context, UsageFilter) (*dto.StatisticsSnapshot, error)
	GetUsageOverview(context.Context, UsageFilter) (*UsageOverviewSnapshot, error)
	ListUsageEvents(context.Context, UsageFilter) (*UsageEventsPage, error)
	ListUsageEventFilterOptions(context.Context, UsageFilter) (*UsageEventFilterOptions, error)
	ListUsageCredentialStats(context.Context, UsageFilter) ([]UsageCredentialStat, error)
	GetUsageAnalysis(context.Context, UsageFilter) (*UsageAnalysisSnapshot, error)
}
