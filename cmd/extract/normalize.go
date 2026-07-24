package main

import (
	"fmt"
	"strings"

	"oddities/database/generated"
)

type normalizedUsage struct {
	mode        generated.UsageMode
	defaultSlot *generated.WearSlot
}

var rarityAliases = map[string]generated.Rarity{
	"common":    generated.RarityCommon,
	"uncommon":  generated.RarityUncommon,
	"rare":      generated.RarityRare,
	"very rare": generated.RarityVeryRare,
	"very_rare": generated.RarityVeryRare,
	"legendary": generated.RarityLegendary,
	"artifact":  generated.RarityArtifact,
	"varies":    generated.RarityVaries,
}

var sourceItemTypeAliases = map[string]generated.SourceItemType{
	"wondrous item": generated.SourceItemTypeWondrousItem,
	"wondrous_item": generated.SourceItemTypeWondrousItem,
	"weapon":        generated.SourceItemTypeWeapon,
	"armor":         generated.SourceItemTypeArmor,
	"potion":        generated.SourceItemTypePotion,
	"ring":          generated.SourceItemTypeRing,
}

var usageAliases = map[string]normalizedUsage{
	"worn":        {mode: generated.UsageModeWorn},
	"held":        {mode: generated.UsageModeHeld},
	"portable":    {mode: generated.UsageModePortable},
	"consumed":    {mode: generated.UsageModeConsumed},
	"held weapon": {mode: generated.UsageModeHeld},
	"held armor":  {mode: generated.UsageModeHeld},
	"worn weapon": {mode: generated.UsageModeWorn},
	"worn armor":  {mode: generated.UsageModeWorn},
	"cloak":       {mode: generated.UsageModeWorn, defaultSlot: wearSlotPointer(generated.WearSlotOuterwear)},
	"ring":        {mode: generated.UsageModeWorn, defaultSlot: wearSlotPointer(generated.WearSlotFinger)},
}

var wearSlotAliases = map[string]generated.WearSlot{
	"head":      generated.WearSlotHead,
	"neck":      generated.WearSlotNeck,
	"torso":     generated.WearSlotTorso,
	"outerwear": generated.WearSlotOuterwear,
	"hands":     generated.WearSlotHands,
	"feet":      generated.WearSlotFeet,
	"finger":    generated.WearSlotFinger,
}

var effectCategoryAliases = map[string]generated.EffectCategory{
	"offensive": generated.EffectCategoryOffensive,
	"offense":   generated.EffectCategoryOffensive,
	"offence":   generated.EffectCategoryOffensive,
	"defensive": generated.EffectCategoryDefensive,
	"defense":   generated.EffectCategoryDefensive,
	"defence":   generated.EffectCategoryDefensive,
	"utility":   generated.EffectCategoryUtility,
}

func Normalize(raw RawCandidate) (NormalizedCandidate, []ValidationIssue) {
	normalized := NormalizedCandidate{
		Raw:           raw,
		ReviewReasons: append([]string(nil), raw.ReviewReasons...),
		NeedsReview:   raw.Confidence < 0.85 || len(raw.ReviewReasons) > 0 || raw.ReviewKind == ReviewKindSourceAmbiguity,
		Effects:       make([]NormalizedEffect, len(raw.Effects)),
	}
	issues := make([]ValidationIssue, 0)

	if value, found := rarityAliases[normalizationKey(raw.RarityRaw)]; found {
		normalized.Rarity = value
	} else {
		issues = append(issues, unknownNormalizationIssue("rarity", raw.RarityRaw))
	}

	if value, found := sourceItemTypeAliases[normalizationKey(raw.SourceItemTypeRaw)]; found {
		normalized.SourceItemType = value
	} else {
		issues = append(issues, unknownNormalizationIssue("source_item_type", raw.SourceItemTypeRaw))
	}

	usageKey := normalizationKey(raw.UsageModeRaw)
	usageValue, usageFound := usageAliases[usageKey]
	if usageFound && !usageAliasSupportedBySource(usageKey, normalizationKey(raw.SourceItemTypeRaw)) {
		usageFound = false
	}
	if !usageFound && usageKey == "" {
		usageValue, usageFound = usageFromExplicitItemWording(raw)
	}
	if usageFound {
		normalized.UsageMode = usageValue.mode
		if raw.WearSlotRaw == nil && usageValue.defaultSlot != nil {
			normalized.WearSlot = wearSlotPointer(*usageValue.defaultSlot)
		}
	} else {
		issues = append(issues, unknownNormalizationIssue("usage_mode", raw.UsageModeRaw))
	}

	if raw.WearSlotRaw != nil {
		if value, found := wearSlotAliases[normalizationKey(*raw.WearSlotRaw)]; found {
			normalized.WearSlot = wearSlotPointer(value)
		} else {
			issues = append(issues, unknownNormalizationIssue("wear_slot", *raw.WearSlotRaw))
		}
	}

	for index, effect := range raw.Effects {
		normalized.Effects[index].Description = effect.Description
		if value, found := effectCategoryAliases[normalizationKey(effect.CategoryRaw)]; found {
			normalized.Effects[index].Category = value
			continue
		}
		issues = append(issues, unknownNormalizationIssue("effect_category", effect.CategoryRaw))
	}

	return normalized, issues
}

func usageFromExplicitItemWording(raw RawCandidate) (normalizedUsage, bool) {
	if normalizationKey(raw.SourceItemTypeRaw) == "ring" {
		return usageAliases["ring"], true
	}
	if raw.SourceItemSubtypeRaw != nil && normalizationKey(*raw.SourceItemSubtypeRaw) == "cloak" {
		return usageAliases["cloak"], true
	}
	return normalizedUsage{}, false
}

func usageAliasSupportedBySource(usage, sourceItemType string) bool {
	switch usage {
	case "held weapon", "worn weapon":
		return sourceItemType == "weapon"
	case "held armor", "worn armor":
		return sourceItemType == "armor"
	default:
		return true
	}
}

func normalizationKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func unknownNormalizationIssue(field, value string) ValidationIssue {
	return ValidationIssue{
		Code:        "unknown_" + field,
		Message:     fmt.Sprintf("unknown %s %q", field, value),
		Recoverable: true,
	}
}

func wearSlotPointer(slot generated.WearSlot) *generated.WearSlot {
	return &slot
}
