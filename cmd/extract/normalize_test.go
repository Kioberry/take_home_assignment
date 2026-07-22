package main

import (
	"reflect"
	"testing"

	"oddities/database/generated"
)

func TestNormalizeCanonicalEnumsAndExplicitAliases(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RawCandidate)
		assert func(t *testing.T, got NormalizedCandidate)
	}{
		{
			name: "rarities",
			mutate: func(candidate *RawCandidate) {
				candidate.RarityRaw = "common"
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityCommon {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityCommon)
				}
			},
		},
		{
			name:   "uncommon rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "uncommon" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityUncommon {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityUncommon)
				}
			},
		},
		{
			name:   "rare rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "rare" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityRare {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityRare)
				}
			},
		},
		{
			name:   "canonical very rare rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "very_rare" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityVeryRare {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityVeryRare)
				}
			},
		},
		{
			name:   "very rare alias",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = " Very Rare " },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityVeryRare {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityVeryRare)
				}
			},
		},
		{
			name:   "legendary rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "legendary" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityLegendary {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityLegendary)
				}
			},
		},
		{
			name:   "artifact rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "artifact" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityArtifact {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityArtifact)
				}
			},
		},
		{
			name:   "varies rarity",
			mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "varies" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.Rarity != generated.RarityVaries {
					t.Fatalf("rarity = %q, want %q", got.Rarity, generated.RarityVaries)
				}
			},
		},
		{
			name:   "canonical wondrous item type",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "wondrous_item" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypeWondrousItem {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypeWondrousItem)
				}
			},
		},
		{
			name:   "wondrous item alias",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "Wondrous Item" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypeWondrousItem {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypeWondrousItem)
				}
			},
		},
		{
			name:   "weapon type",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "weapon" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypeWeapon {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypeWeapon)
				}
			},
		},
		{
			name:   "armor type",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "armor" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypeArmor {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypeArmor)
				}
			},
		},
		{
			name:   "potion type",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "potion" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypePotion {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypePotion)
				}
			},
		},
		{
			name:   "ring type",
			mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "ring" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.SourceItemType != generated.SourceItemTypeRing {
					t.Fatalf("source type = %q, want %q", got.SourceItemType, generated.SourceItemTypeRing)
				}
			},
		},
		{
			name: "worn usage and every wear slot",
			mutate: func(candidate *RawCandidate) {
				candidate.UsageModeRaw = "worn"
				candidate.WearSlotRaw = stringPointer("head")
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeWorn || got.WearSlot == nil || *got.WearSlot != generated.WearSlotHead {
					t.Fatalf("usage/slot = %q/%v, want worn/head", got.UsageMode, got.WearSlot)
				}
			},
		},
		{
			name:   "held usage",
			mutate: func(candidate *RawCandidate) { candidate.UsageModeRaw = "held" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeHeld {
					t.Fatalf("usage = %q, want %q", got.UsageMode, generated.UsageModeHeld)
				}
			},
		},
		{
			name:   "portable usage",
			mutate: func(candidate *RawCandidate) { candidate.UsageModeRaw = "portable" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModePortable {
					t.Fatalf("usage = %q, want %q", got.UsageMode, generated.UsageModePortable)
				}
			},
		},
		{
			name:   "consumed usage",
			mutate: func(candidate *RawCandidate) { candidate.UsageModeRaw = "consumed" },
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeConsumed {
					t.Fatalf("usage = %q, want %q", got.UsageMode, generated.UsageModeConsumed)
				}
			},
		},
		{
			name: "cloak alias supplies worn outerwear",
			mutate: func(candidate *RawCandidate) {
				candidate.UsageModeRaw = ""
				candidate.SourceItemSubtypeRaw = stringPointer("cloak")
				candidate.WearSlotRaw = nil
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeWorn || got.WearSlot == nil || *got.WearSlot != generated.WearSlotOuterwear {
					t.Fatalf("usage/slot = %q/%v, want worn/outerwear", got.UsageMode, got.WearSlot)
				}
			},
		},
		{
			name: "ring alias supplies worn finger",
			mutate: func(candidate *RawCandidate) {
				candidate.SourceItemTypeRaw = "ring"
				candidate.UsageModeRaw = ""
				candidate.WearSlotRaw = nil
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeWorn || got.WearSlot == nil || *got.WearSlot != generated.WearSlotFinger {
					t.Fatalf("usage/slot = %q/%v, want worn/finger", got.UsageMode, got.WearSlot)
				}
			},
		},
		{
			name: "explicitly held weapon",
			mutate: func(candidate *RawCandidate) {
				candidate.SourceItemTypeRaw = "weapon"
				candidate.UsageModeRaw = "held weapon"
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeHeld {
					t.Fatalf("usage = %q, want held", got.UsageMode)
				}
			},
		},
		{
			name: "explicitly worn armor",
			mutate: func(candidate *RawCandidate) {
				candidate.SourceItemTypeRaw = "armor"
				candidate.UsageModeRaw = "worn armor"
				candidate.WearSlotRaw = stringPointer("torso")
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.UsageMode != generated.UsageModeWorn || got.WearSlot == nil || *got.WearSlot != generated.WearSlotTorso {
					t.Fatalf("usage/slot = %q/%v, want worn/torso", got.UsageMode, got.WearSlot)
				}
			},
		},
		{
			name: "every remaining wear slot",
			mutate: func(candidate *RawCandidate) {
				candidate.UsageModeRaw = "worn"
				candidate.WearSlotRaw = stringPointer("neck")
			},
			assert: func(t *testing.T, got NormalizedCandidate) {
				if got.WearSlot == nil || *got.WearSlot != generated.WearSlotNeck {
					t.Fatalf("wear slot = %v, want neck", got.WearSlot)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validRawCandidate()
			tc.mutate(&candidate)

			got, issues := Normalize(candidate)
			if len(issues) != 0 {
				t.Fatalf("issues = %#v, want none", issues)
			}
			tc.assert(t, got)
		})
	}
}

func TestNormalizeEffectCategoriesAndPreservesDescriptionBytes(t *testing.T) {
	tests := []struct {
		raw  string
		want generated.EffectCategory
	}{
		{raw: "offensive", want: generated.EffectCategoryOffensive},
		{raw: "offense", want: generated.EffectCategoryOffensive},
		{raw: "defensive", want: generated.EffectCategoryDefensive},
		{raw: "defense", want: generated.EffectCategoryDefensive},
		{raw: "utility", want: generated.EffectCategoryUtility},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			candidate := validRawCandidate()
			candidate.Effects[0].CategoryRaw = tc.raw
			candidate.RawDescription = "  Source\n\tbytes  "
			candidate.Effects[0].Description = "  Effect\n\tbytes  "
			candidate.Limitations[0].Description = "  Limitation\n\tbytes  "

			got, issues := Normalize(candidate)
			if len(issues) != 0 {
				t.Fatalf("issues = %#v, want none", issues)
			}
			if got.Effects[0].Category != tc.want {
				t.Fatalf("effect category = %q, want %q", got.Effects[0].Category, tc.want)
			}
			if got.Raw.RawDescription != candidate.RawDescription || got.Effects[0].Description != candidate.Effects[0].Description || got.Raw.Limitations[0].Description != candidate.Limitations[0].Description {
				t.Fatalf("descriptions changed: %#v", got)
			}
		})
	}
}

func TestNormalizeEveryWearSlot(t *testing.T) {
	tests := []struct {
		raw  string
		want generated.WearSlot
	}{
		{raw: "head", want: generated.WearSlotHead},
		{raw: "neck", want: generated.WearSlotNeck},
		{raw: "torso", want: generated.WearSlotTorso},
		{raw: "outerwear", want: generated.WearSlotOuterwear},
		{raw: "hands", want: generated.WearSlotHands},
		{raw: "feet", want: generated.WearSlotFeet},
		{raw: "finger", want: generated.WearSlotFinger},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			candidate := validRawCandidate()
			candidate.UsageModeRaw = "worn"
			candidate.WearSlotRaw = stringPointer(tc.raw)

			got, issues := Normalize(candidate)
			if len(issues) != 0 {
				t.Fatalf("issues = %#v, want none", issues)
			}
			if got.WearSlot == nil || *got.WearSlot != tc.want {
				t.Fatalf("wear slot = %v, want %q", got.WearSlot, tc.want)
			}
		})
	}
}

func TestNormalizeUnknownOrAmbiguousValuesAreRecoverable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RawCandidate)
		code   string
	}{
		{name: "unknown source item type", mutate: func(candidate *RawCandidate) { candidate.SourceItemTypeRaw = "relic" }, code: "unknown_source_item_type"},
		{name: "unknown rarity", mutate: func(candidate *RawCandidate) { candidate.RarityRaw = "mythic" }, code: "unknown_rarity"},
		{name: "ambiguous weapon usage", mutate: func(candidate *RawCandidate) { candidate.UsageModeRaw = "weapon" }, code: "unknown_usage_mode"},
		{name: "mismatched held weapon usage", mutate: func(candidate *RawCandidate) {
			candidate.SourceItemTypeRaw = "potion"
			candidate.UsageModeRaw = "held weapon"
		}, code: "unknown_usage_mode"},
		{name: "unknown wear slot", mutate: func(candidate *RawCandidate) { candidate.WearSlotRaw = stringPointer("wrist") }, code: "unknown_wear_slot"},
		{name: "unknown effect category", mutate: func(candidate *RawCandidate) { candidate.Effects[0].CategoryRaw = "navigation" }, code: "unknown_effect_category"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validRawCandidate()
			tc.mutate(&candidate)

			_, issues := Normalize(candidate)
			if len(issues) != 1 {
				t.Fatalf("issues = %#v, want one", issues)
			}
			if issues[0].Code != tc.code || !issues[0].Recoverable {
				t.Fatalf("issue = %#v, want recoverable %q", issues[0], tc.code)
			}
		})
	}
}

func TestNormalizeNeedsReviewForLowConfidenceOrReasons(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		reasons    []string
		want       bool
	}{
		{name: "confident candidate", confidence: 0.85, want: false},
		{name: "below threshold", confidence: 0.849, want: true},
		{name: "review reason", confidence: 0.99, reasons: []string{"OCR ambiguity"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validRawCandidate()
			candidate.Confidence = tc.confidence
			candidate.ReviewReasons = tc.reasons

			got, issues := Normalize(candidate)
			if len(issues) != 0 {
				t.Fatalf("issues = %#v, want none", issues)
			}
			if got.NeedsReview != tc.want || !reflect.DeepEqual(got.ReviewReasons, tc.reasons) {
				t.Fatalf("needs review/reasons = %v/%#v, want %v/%#v", got.NeedsReview, got.ReviewReasons, tc.want, tc.reasons)
			}
		})
	}
}

func validRawCandidate() RawCandidate {
	effectIndex := 0
	return RawCandidate{
		Name:              "Arcane Compass",
		SourcePages:       []int{1},
		SourceItemTypeRaw: "wondrous item",
		RarityRaw:         "rare",
		UsageModeRaw:      "portable",
		RawDescription:    "Points toward a named destination.",
		Effects: []RawEffect{{
			CategoryRaw: "utility",
			Description: "Points toward a named destination.",
		}},
		Limitations: []RawLimitation{{
			EffectIndex: &effectIndex,
			Description: "Requires a destination name.",
		}},
		Confidence: 0.9,
	}
}
