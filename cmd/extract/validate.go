package main

import (
	"fmt"
	"math"
	"strings"

	"oddities/database/generated"
)

func Validate(candidate NormalizedCandidate, pageLimit int) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(code, message string) {
		issues = append(issues, ValidationIssue{Code: code, Message: message, Recoverable: false})
	}

	if strings.TrimSpace(candidate.Raw.Name) == "" {
		add("blank_name", "item name must not be blank")
	}
	if strings.TrimSpace(candidate.Raw.RawDescription) == "" {
		add("blank_raw_description", "raw description must not be blank")
	}
	if candidate.Raw.SourceItemSubtypeRaw != nil && strings.TrimSpace(*candidate.Raw.SourceItemSubtypeRaw) == "" {
		add("blank_source_item_subtype", "source item subtype must not be blank when present")
	}
	if candidate.Raw.AttunementRequirement != nil && strings.TrimSpace(*candidate.Raw.AttunementRequirement) == "" {
		add("blank_attunement_requirement", "attunement requirement must not be blank when present")
	}
	if math.IsNaN(candidate.Raw.Confidence) || candidate.Raw.Confidence < 0 || candidate.Raw.Confidence > 1 {
		add("invalid_confidence", "confidence must be within [0,1]")
	}

	validateSourcePages(candidate.Raw.SourcePages, pageLimit, add)
	validateCanonicalValues(candidate, add)

	if candidate.UsageMode == generated.UsageModeWorn && candidate.WearSlot == nil {
		add("worn_without_wear_slot", "worn items require a wear slot")
	}
	if candidate.UsageMode != generated.UsageModeWorn && candidate.WearSlot != nil {
		add("non_worn_with_wear_slot", "only worn items may have a wear slot")
	}
	if !candidate.Raw.RequiresAttunement && candidate.Raw.AttunementRequirement != nil {
		add("unattuned_with_requirement", "unattuned items must not have an attunement requirement")
	}

	for index, effect := range candidate.Effects {
		if strings.TrimSpace(effect.Description) == "" {
			add("blank_effect_description", fmt.Sprintf("effect %d description must not be blank", index))
		}
	}
	for index, limitation := range candidate.Raw.Limitations {
		if strings.TrimSpace(limitation.Description) == "" {
			add("blank_limitation_description", fmt.Sprintf("limitation %d description must not be blank", index))
		}
		if limitation.EffectIndex != nil && (*limitation.EffectIndex < 0 || *limitation.EffectIndex >= len(candidate.Effects)) {
			add("limitation_effect_index_out_of_range", fmt.Sprintf("limitation %d effect index is outside the effects slice", index))
		}
	}

	return issues
}

func validateSourcePages(pages []int, pageLimit int, add func(string, string)) {
	if len(pages) == 0 {
		add("missing_source_pages", "at least one source page is required")
		return
	}
	if pageLimit < 1 {
		add("invalid_page_limit", "page limit must be at least one")
		return
	}

	for index, page := range pages {
		if page < 1 || page > pageLimit {
			add("source_page_out_of_range", fmt.Sprintf("source page %d is outside 1..%d", page, pageLimit))
		}
		if index > 0 && page != pages[index-1]+1 {
			add("non_contiguous_source_pages", "source pages must be ascending and contiguous")
		}
	}
}

func validateCanonicalValues(candidate NormalizedCandidate, add func(string, string)) {
	if !isCanonicalRarity(candidate.Rarity) {
		add("invalid_rarity", "rarity is not a generated enum value")
	}
	if !isCanonicalSourceItemType(candidate.SourceItemType) {
		add("invalid_source_item_type", "source item type is not a generated enum value")
	}
	if !isCanonicalUsageMode(candidate.UsageMode) {
		add("invalid_usage_mode", "usage mode is not a generated enum value")
	}
	if candidate.WearSlot != nil {
		if !isCanonicalWearSlot(*candidate.WearSlot) {
			add("invalid_wear_slot", "wear slot is not a generated enum value")
		}
	}
	for index, effect := range candidate.Effects {
		if !isCanonicalEffectCategory(effect.Category) {
			add("invalid_effect_category", fmt.Sprintf("effect %d category is not a generated enum value", index))
		}
	}
}

func isCanonicalRarity(value generated.Rarity) bool {
	switch value {
	case generated.RarityCommon, generated.RarityUncommon, generated.RarityRare, generated.RarityVeryRare, generated.RarityLegendary, generated.RarityArtifact, generated.RarityVaries:
		return true
	default:
		return false
	}
}

func isCanonicalSourceItemType(value generated.SourceItemType) bool {
	switch value {
	case generated.SourceItemTypeWondrousItem, generated.SourceItemTypeWeapon, generated.SourceItemTypeArmor, generated.SourceItemTypePotion, generated.SourceItemTypeRing:
		return true
	default:
		return false
	}
}

func isCanonicalUsageMode(value generated.UsageMode) bool {
	switch value {
	case generated.UsageModeWorn, generated.UsageModeHeld, generated.UsageModePortable, generated.UsageModeConsumed:
		return true
	default:
		return false
	}
}

func isCanonicalWearSlot(value generated.WearSlot) bool {
	switch value {
	case generated.WearSlotHead, generated.WearSlotNeck, generated.WearSlotTorso, generated.WearSlotOuterwear, generated.WearSlotHands, generated.WearSlotFeet, generated.WearSlotFinger:
		return true
	default:
		return false
	}
}

func isCanonicalEffectCategory(value generated.EffectCategory) bool {
	switch value {
	case generated.EffectCategoryOffensive, generated.EffectCategoryDefensive, generated.EffectCategoryUtility:
		return true
	default:
		return false
	}
}
