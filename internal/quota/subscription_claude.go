package quota

import "strings"

func resolveClaudeSubscription(result any) *SubscriptionInfo {
	var profile *ClaudeProfileResponse
	switch value := result.(type) {
	case ClaudeResult:
		profile = value.Profile
	case *ClaudeResult:
		if value != nil {
			profile = value.Profile
		}
	}
	if profile == nil {
		return nil
	}

	// 套餐优先级与上游管理中心保持一致，且只有两个 flag 都明确为 false 时才判定 Free。
	if profile.Account != nil && profile.Account.HasClaudeMax != nil && *profile.Account.HasClaudeMax {
		return newClaudeSubscription("max")
	}
	if profile.Account != nil && profile.Account.HasClaudePro != nil && *profile.Account.HasClaudePro {
		return newClaudeSubscription("pro")
	}
	if profile.Organization != nil &&
		strings.EqualFold(strings.TrimSpace(profile.Organization.OrganizationType), "claude_team") &&
		strings.EqualFold(strings.TrimSpace(profile.Organization.SubscriptionStatus), "active") {
		return newClaudeSubscription("team")
	}
	if profile.Account != nil &&
		profile.Account.HasClaudeMax != nil && !*profile.Account.HasClaudeMax &&
		profile.Account.HasClaudePro != nil && !*profile.Account.HasClaudePro {
		return newClaudeSubscription("free")
	}
	return nil
}

func newClaudeSubscription(plan string) *SubscriptionInfo {
	return &SubscriptionInfo{Provider: "claude", Plan: plan}
}
