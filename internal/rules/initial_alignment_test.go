package rules

import (
	"path/filepath"
	"testing"
)

func TestInitialAlignmentDefaultsDisabledAndRoundTripsManual(t *testing.T) {
	for _, set := range []*RuleSet{DefaultRuleSet(), DefaultFieldRuleSet()} {
		for _, rule := range set.Rules {
			if rule.InitialAlignment.EffectivePolicy() != AlignmentDisabled {
				t.Fatalf("default alignment enabled: %s", rule.ID)
			}
		}
	}
	set := DefaultRuleSet()
	set.Rules[0].InitialAlignment.Policy = AlignmentManual
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := SaveFile(path, *set); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Rules[0].InitialAlignment.EffectivePolicy() != AlignmentManual || loaded.Rules[1].InitialAlignment.EffectivePolicy() != AlignmentDisabled {
		t.Fatal("optional policy was lost or applied to other rules")
	}
}

func TestInitialAlignmentRejectsAutomaticPolicies(t *testing.T) {
	for _, policy := range []string{"AUTO", "true", "manual", "ON_START", "ON_REGISTER", " "} {
		set := DefaultRuleSet()
		set.Rules[0].InitialAlignment.Policy = policy
		if err := set.Validate(); err == nil {
			t.Fatalf("unsafe alignment policy accepted: %q", policy)
		}
	}
}
