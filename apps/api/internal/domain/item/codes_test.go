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

	substitutabilities := []domainitem.Substitutability{
		domainitem.SubstitutabilityNone,
		domainitem.SubstitutabilityPartial,
		domainitem.SubstitutabilityFull,
		domainitem.SubstitutabilityUnknown,
	}
	for _, substitutability := range substitutabilities {
		if substitutability.Label() == "" {
			t.Errorf("Substitutability %q has empty label", substitutability)
		}
		if _, err := domainitem.NewSubstitutability(substitutability.String()); err != nil {
			t.Errorf("NewSubstitutability(%q) returned error: %v", substitutability, err)
		}
	}

	mobilityClasses := []domainitem.MobilityClass{
		domainitem.MobilityClassWorn,
		domainitem.MobilityClassPocket,
		domainitem.MobilityClassDailyBag,
		domainitem.MobilityClassOnDemand,
		domainitem.MobilityClassSelfCarry,
		domainitem.MobilityClassParcel,
		domainitem.MobilityClassMover,
		domainitem.MobilityClassDisposeRebuy,
		domainitem.MobilityClassFixed,
	}
	for _, class := range mobilityClasses {
		if class.Label() == "" {
			t.Errorf("MobilityClass %q has empty label", class)
		}
		if _, err := domainitem.NewMobilityClass(class.String()); err != nil {
			t.Errorf("NewMobilityClass(%q) returned error: %v", class, err)
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
	if _, err := domainitem.NewSubstitutability("maybe"); err == nil {
		t.Error("NewSubstitutability(maybe) returned nil error, want error")
	}
	if _, err := domainitem.NewMobilityClass("truck"); err == nil {
		t.Error("NewMobilityClass(truck) returned nil error, want error")
	}
}
