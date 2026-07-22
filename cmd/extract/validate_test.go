package main

import (
	"strings"
	"testing"

	"oddities/database/generated"
)

func TestValidateStructuralInvariants(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NormalizedCandidate)
		code   string
	}{
		{name: "blank name", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.Name = " \t" }, code: "blank_name"},
		{name: "blank raw description", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.RawDescription = "\n" }, code: "blank_raw_description"},
		{name: "confidence below zero", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.Confidence = -0.01 }, code: "invalid_confidence"},
		{name: "confidence above one", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.Confidence = 1.01 }, code: "invalid_confidence"},
		{name: "non-contiguous pages", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.SourcePages = []int{1, 3} }, code: "non_contiguous_source_pages"},
		{name: "page below range", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.SourcePages = []int{0} }, code: "source_page_out_of_range"},
		{name: "page above range", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.SourcePages = []int{4} }, code: "source_page_out_of_range"},
		{name: "worn without slot", mutate: func(candidate *NormalizedCandidate) {
			candidate.UsageMode, candidate.WearSlot = generated.UsageModeWorn, nil
		}, code: "worn_without_wear_slot"},
		{name: "non-worn with slot", mutate: func(candidate *NormalizedCandidate) {
			slot := generated.WearSlotHead
			candidate.UsageMode, candidate.WearSlot = generated.UsageModeHeld, &slot
		}, code: "non_worn_with_wear_slot"},
		{name: "unattuned with requirement", mutate: func(candidate *NormalizedCandidate) {
			requirement := "by a wizard"
			candidate.Raw.RequiresAttunement, candidate.Raw.AttunementRequirement = false, &requirement
		}, code: "unattuned_with_requirement"},
		{name: "blank effect description", mutate: func(candidate *NormalizedCandidate) { candidate.Effects[0].Description = " \n" }, code: "blank_effect_description"},
		{name: "blank limitation description", mutate: func(candidate *NormalizedCandidate) { candidate.Raw.Limitations[0].Description = "\t" }, code: "blank_limitation_description"},
		{name: "limitation below effect range", mutate: func(candidate *NormalizedCandidate) {
			index := -1
			candidate.Raw.Limitations[0].EffectIndex = &index
		}, code: "limitation_effect_index_out_of_range"},
		{name: "limitation above effect range", mutate: func(candidate *NormalizedCandidate) {
			index := len(candidate.Effects)
			candidate.Raw.Limitations[0].EffectIndex = &index
		}, code: "limitation_effect_index_out_of_range"},
		{name: "invalid canonical rarity", mutate: func(candidate *NormalizedCandidate) { candidate.Rarity = generated.Rarity("mythic") }, code: "invalid_rarity"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validNormalizedCandidate(t)
			tc.mutate(&candidate)

			issues := Validate(candidate, 3)
			if len(issues) != 1 {
				t.Fatalf("issues = %#v, want one", issues)
			}
			if issues[0].Code != tc.code || issues[0].Recoverable {
				t.Fatalf("issue = %#v, want non-recoverable %q", issues[0], tc.code)
			}
		})
	}
}

func TestValidateAcceptsValidCandidate(t *testing.T) {
	if issues := Validate(validNormalizedCandidate(t), 1); len(issues) != 0 {
		t.Fatalf("issues = %#v, want none", issues)
	}
}

func TestValidateDoesNotRewriteDescriptions(t *testing.T) {
	candidate := validNormalizedCandidate(t)
	candidate.Raw.RawDescription = "  Source\n\tbytes  "
	candidate.Effects[0].Description = "  Effect\n\tbytes  "
	candidate.Raw.Limitations[0].Description = "  Limitation\n\tbytes  "

	_ = Validate(candidate, 1)
	if candidate.Raw.RawDescription != "  Source\n\tbytes  " || candidate.Effects[0].Description != "  Effect\n\tbytes  " || candidate.Raw.Limitations[0].Description != "  Limitation\n\tbytes  " {
		t.Fatalf("Validate rewrote descriptions: %#v", candidate)
	}
}

func validNormalizedCandidate(t *testing.T) NormalizedCandidate {
	t.Helper()
	normalized, issues := Normalize(validRawCandidate())
	if len(issues) != 0 {
		t.Fatalf("Normalize issues = %#v", issues)
	}
	return normalized
}

func TestValidateReportsNonBlankDescriptionsOnly(t *testing.T) {
	candidate := validNormalizedCandidate(t)
	candidate.Raw.RawDescription = strings.Repeat(" ", 3)
	issues := Validate(candidate, 1)
	if len(issues) != 1 || issues[0].Code != "blank_raw_description" {
		t.Fatalf("issues = %#v, want blank raw description", issues)
	}
}
