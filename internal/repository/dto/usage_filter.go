package dto

import "time"

// UsageQueryFilter 是仓储层的 usage 查询条件。
type UsageQueryFilter struct {
	Range     string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Page      int
	PageSize  int
	Offset    int
	Model     string
	Source    string
	AuthIndex string
	AuthType  string
	Provider  string
	Result    string
}

const DefaultUsageEventsLimit = 100

// UsageEventsPageRecord 是 usage events 列表的仓储查询结果。
type UsageEventsPageRecord struct {
	Events     []UsageEventRecord
	Models     []string
	Sources    []string
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}

// UsageEventFilterOptionsRecord 是 usage events 筛选项的仓储查询结果。
type UsageEventFilterOptionsRecord struct {
	Models  []string
	Sources []string
}

// UsageEventRecord 是单条 usage event 的查询结果。
type UsageEventRecord struct {
	ID              uint
	Timestamp       time.Time
	APIGroupKey     string
	Model           string
	AuthType        string
	Provider        string
	Source          string
	AuthIndex       string
	Failed          bool
	LatencyMS       int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

// UsageCredentialStatRecord 是 credential 聚合统计结果。
type UsageCredentialStatRecord struct {
	Source       string
	AuthIndex    string
	Failed       bool
	RequestCount int64
}

// UsageAnalysisModelStatRecord 是按模型聚合的分析结果。
type UsageAnalysisModelStatRecord struct {
	Model              string
	TotalRequests      int64
	SuccessCount       int64
	FailureCount       int64
	InputTokens        int64
	OutputTokens       int64
	ReasoningTokens    int64
	CachedTokens       int64
	TotalTokens        int64
	TotalLatencyMS     int64
	LatencySampleCount int64
}

// UsageAnalysisAPIStatRecord 是按 API 聚合的分析结果。
type UsageAnalysisAPIStatRecord struct {
	APIGroupKey     string
	DisplayName     string
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
	Models          []UsageAnalysisModelStatRecord `gorm:"-"`
}

// UsageOverviewSummaryRecord 是 overview 的 summary 聚合结果。
type UsageOverviewSummaryRecord struct {
	RequestCount    int64
	TokenCount      int64
	WindowMinutes   int64
	RPM             float64
	TPM             float64
	TotalCost       float64
	CostAvailable   bool
	CachedTokens    int64
	ReasoningTokens int64
}

// UsageOverviewSeriesRecord 是 overview 的 series 聚合结果。
type UsageOverviewSeriesRecord struct {
	Requests        map[string]int64
	Tokens          map[string]int64
	RPM             map[string]float64
	TPM             map[string]float64
	Cost            map[string]float64
	InputTokens     map[string]int64
	OutputTokens    map[string]int64
	CachedTokens    map[string]int64
	ReasoningTokens map[string]int64
	Models          map[string]UsageOverviewSeriesRecord
}

// UsageOverviewHealthBlockRecord 是 overview health 的单个时间块。
type UsageOverviewHealthBlockRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Success   int64
	Failure   int64
	Rate      float64
}

// UsageOverviewHealthRecord 是 overview health 的聚合结果。
type UsageOverviewHealthRecord struct {
	TotalSuccess  int64
	TotalFailure  int64
	SuccessRate   float64
	Rows          int
	Columns       int
	BucketSeconds int64
	WindowStart   time.Time
	WindowEnd     time.Time
	BlockDetails  []UsageOverviewHealthBlockRecord
}

// UsageOverviewRecord 是仓储层的完整 usage overview 结果。
type UsageOverviewRecord struct {
	Usage        *StatisticsSnapshot
	Summary      UsageOverviewSummaryRecord
	Series       UsageOverviewSeriesRecord
	HourlySeries UsageOverviewSeriesRecord
	DailySeries  UsageOverviewSeriesRecord
	Health       UsageOverviewHealthRecord
}
