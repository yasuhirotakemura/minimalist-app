package item_test

import (
	"testing"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

func TestNewItemKind(t *testing.T) {
	// 空文字は既定値とする。
	kind, err := domainitem.NewItemKind("")
	if err != nil {
		t.Fatalf("NewItemKind(\"\") returned error: %v", err)
	}
	if kind != domainitem.DefaultItemKind {
		t.Errorf("kind = %q, want %q", kind, domainitem.DefaultItemKind)
	}

	if _, err := domainitem.NewItemKind("rental"); err == nil {
		t.Fatal("NewItemKind(rental) returned nil error, want error")
	}
}

func TestCodeLabels_全ての値にlabelがある(t *testing.T) {
	necessityLevels := []domainitem.NecessityLevel{
		domainitem.NecessityLevelEssential,
		domainitem.NecessityLevelImportant,
		domainitem.NecessityLevelOptional,
		domainitem.NecessityLevelUndecided,
		domainitem.NecessityLevelUnnecessary,
	}
	for _, level := range necessityLevels {
		if level.Label() == "" {
			t.Errorf("NecessityLevel %q has empty label", level)
		}
		if _, err := domainitem.NewNecessityLevel(level.String()); err != nil {
			t.Errorf("NewNecessityLevel(%q) returned error: %v", level, err)
		}
	}

	usageFrequencies := []domainitem.UsageFrequency{
		domainitem.UsageFrequencyDaily,
		domainitem.UsageFrequencyWeekly,
		domainitem.UsageFrequencyMonthly,
		domainitem.UsageFrequencyQuarterly,
		domainitem.UsageFrequencyYearly,
		domainitem.UsageFrequencyRarely,
		domainitem.UsageFrequencyNever,
	}
	for _, frequency := range usageFrequencies {
		if frequency.Label() == "" {
			t.Errorf("UsageFrequency %q has empty label", frequency)
		}
		if _, err := domainitem.NewUsageFrequency(frequency.String()); err != nil {
			t.Errorf("NewUsageFrequency(%q) returned error: %v", frequency, err)
		}
	}
}

func TestCodes_未知の値を拒否する(t *testing.T) {
	if _, err := domainitem.NewNecessityLevel(""); err == nil {
		t.Error("NewNecessityLevel(\"\") returned nil error, want error")
	}
	if _, err := domainitem.NewUsageFrequency("sometimes"); err == nil {
		t.Error("NewUsageFrequency(sometimes) returned nil error, want error")
	}
}

func TestCodesInOrder_全ての値を表示順で返す(t *testing.T) {
	levels := domainitem.NecessityLevelsInOrder()
	if len(levels) != 5 {
		t.Fatalf("len(NecessityLevelsInOrder()) = %d, want 5", len(levels))
	}
	if levels[0] != domainitem.NecessityLevelEssential {
		t.Errorf("levels[0] = %q, want %q", levels[0], domainitem.NecessityLevelEssential)
	}

	frequencies := domainitem.UsageFrequenciesInOrder()
	if len(frequencies) != 7 {
		t.Fatalf("len(UsageFrequenciesInOrder()) = %d, want 7", len(frequencies))
	}
	if frequencies[0] != domainitem.UsageFrequencyDaily {
		t.Errorf("frequencies[0] = %q, want %q", frequencies[0], domainitem.UsageFrequencyDaily)
	}

	// 返り値を書き換えても内部の定義順へ影響しない。
	levels[0] = domainitem.NecessityLevelUnnecessary
	if domainitem.NecessityLevelsInOrder()[0] != domainitem.NecessityLevelEssential {
		t.Error("NecessityLevelsInOrder() returned a mutable shared slice")
	}
}
