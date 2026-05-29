package helper

import (
	"testing"

	"cpa-usage-keeper/internal/entities"
)

func TestCPAAPIKeyDisplayLabelPrefersAlias(t *testing.T) {
	row := entities.CPAAPIKey{KeyAlias: "  Production  ", DisplayKey: "sk-*********123456"}

	if got := CPAAPIKeyDisplayLabel(row); got != "Production" {
		t.Fatalf("expected alias label, got %q", got)
	}
}

func TestCPAAPIKeyDisplayLabelFallsBackToDisplayKey(t *testing.T) {
	row := entities.CPAAPIKey{DisplayKey: "sk-*********123456"}

	if got := CPAAPIKeyDisplayLabel(row); got != "sk-*********123456" {
		t.Fatalf("expected display key fallback, got %q", got)
	}
}
