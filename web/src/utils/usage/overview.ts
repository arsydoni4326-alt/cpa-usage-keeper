import type { UsageTimeRange } from '@/lib/types';

export const getOverviewDisplayLoading = ({ loading, hasUsage }: { loading: boolean; hasUsage: boolean }) => loading && !hasUsage;

export const getCurrentOverviewUsage = <T>(
  usage: T | null,
  currentQueryKey: string | null,
  loadedQueryKey: string | null,
): T | null => {
  if (!usage || !currentQueryKey || loadedQueryKey !== currentQueryKey) {
    return null;
  }
  return usage;
};

export const getDailyAveragePanelUsage = <T>(
  currentUsage: T | null,
  fallbackUsage: T | null,
  reserveVisible: boolean,
): T | null => currentUsage ?? (reserveVisible ? fallbackUsage : null);

const dateOnlyPattern = /^\d{4}-\d{2}-\d{2}$/;

export const isDailyAverageRange = ({
  range,
  customStart,
  customEnd,
}: {
  range: UsageTimeRange;
  customStart?: string;
  customEnd?: string;
}): boolean => {
  if (range === '7d' || range === '30d') {
    return true;
  }
  if (range !== 'custom') {
    return false;
  }
  const start = customStart?.trim() ?? '';
  const end = customEnd?.trim() ?? '';
  return dateOnlyPattern.test(start) && dateOnlyPattern.test(end) && start < end;
};
