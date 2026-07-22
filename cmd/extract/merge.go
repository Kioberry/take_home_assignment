package main

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type candidateMergeGroup struct {
	candidate       RawCandidate
	hasContinuation bool
}

// CandidateKey returns the stable in-run identity for an extracted candidate.
// Source pages are sorted for comparison, while the candidate itself is not
// modified.
func CandidateKey(candidate RawCandidate) string {
	pages := uniqueSortedPages(candidate.SourcePages)
	pageParts := make([]string, 0, len(pages))
	for _, page := range pages {
		pageParts = append(pageParts, strconv.Itoa(page))
	}
	descriptionHash := sha256.Sum256([]byte(normalizeWhitespace(candidate.RawDescription)))
	return strings.Join([]string{
		normalizeIdentity(candidate.Name),
		strings.Join(pageParts, ","),
		fmt.Sprintf("%x", descriptionHash),
	}, "|")
}

// MergeCandidates deduplicates candidates from overlapping batches and merges
// only source fragments that explicitly indicate continuation.
func MergeCandidates(batches [][]RawCandidate) (merged []RawCandidate, conflicts []ValidationIssue) {
	flattened := make([]RawCandidate, 0)
	for _, batch := range batches {
		for _, candidate := range batch {
			flattened = append(flattened, cloneCandidate(candidate))
		}
	}

	// Page order makes fragment descriptions and output order independent of
	// which overlapping batch happened to return first.
	sort.SliceStable(flattened, func(i, j int) bool {
		return candidatePageStart(flattened[i]) < candidatePageStart(flattened[j])
	})

	groups := make([]candidateMergeGroup, 0, len(flattened))
	keyIndexes := make(map[string]int, len(flattened))
	for _, candidate := range flattened {
		key := CandidateKey(candidate)
		if index, ok := keyIndexes[key]; ok {
			mergeCandidateInto(&groups[index], candidate, &conflicts)
			continue
		}

		index := -1
		for i := range groups {
			if canMergeCandidates(groups[i], candidate) {
				index = i
				break
			}
		}
		if index < 0 {
			groups = append(groups, candidateMergeGroup{
				candidate:       candidate,
				hasContinuation: candidate.Continuation,
			})
			keyIndexes[key] = len(groups) - 1
			continue
		}

		mergeCandidateInto(&groups[index], candidate, &conflicts)
		keyIndexes[key] = index
	}

	merged = make([]RawCandidate, 0, len(groups))
	for _, group := range groups {
		merged = append(merged, group.candidate)
	}
	return merged, conflicts
}

func canMergeCandidates(group candidateMergeGroup, candidate RawCandidate) bool {
	return normalizeIdentity(group.candidate.Name) == normalizeIdentity(candidate.Name) &&
		pagesTouch(group.candidate.SourcePages, candidate.SourcePages) &&
		(group.hasContinuation || candidate.Continuation)
}

func mergeCandidateInto(group *candidateMergeGroup, candidate RawCandidate, conflicts *[]ValidationIssue) {
	existing := &group.candidate
	mergeStringField(&existing.SourceItemTypeRaw, candidate.SourceItemTypeRaw, "source_item_type_raw", existing.Name, conflicts)
	mergePointerStringField(&existing.SourceItemSubtypeRaw, candidate.SourceItemSubtypeRaw, "source_item_subtype_raw", existing.Name, conflicts)
	mergeStringField(&existing.RarityRaw, candidate.RarityRaw, "rarity_raw", existing.Name, conflicts)
	mergeStringField(&existing.UsageModeRaw, candidate.UsageModeRaw, "usage_mode_raw", existing.Name, conflicts)
	mergePointerStringField(&existing.WearSlotRaw, candidate.WearSlotRaw, "wear_slot_raw", existing.Name, conflicts)
	if existing.RequiresAttunement != candidate.RequiresAttunement {
		appendMergeConflict(conflicts, existing.Name, "requires_attunement", fmt.Sprintf("%t", existing.RequiresAttunement), fmt.Sprintf("%t", candidate.RequiresAttunement))
	}
	mergePointerStringField(&existing.AttunementRequirement, candidate.AttunementRequirement, "attunement_requirement", existing.Name, conflicts)

	existing.SourcePages = unionPages(existing.SourcePages, candidate.SourcePages)
	existing.RawDescription = mergeDescription(existing.RawDescription, candidate.RawDescription)
	existing.Effects = unionEffects(existing.Effects, candidate.Effects)
	existing.Limitations = unionLimitations(existing.Limitations, candidate.Limitations)
	if candidate.Confidence < existing.Confidence {
		existing.Confidence = candidate.Confidence
	}
	existing.ReviewReasons = unionStrings(existing.ReviewReasons, candidate.ReviewReasons)
	existing.Continuation = existing.Continuation && candidate.Continuation
	group.hasContinuation = group.hasContinuation || candidate.Continuation
}

func mergeStringField(existing *string, incoming, field, candidateName string, conflicts *[]ValidationIssue) {
	if strings.TrimSpace(*existing) == "" {
		*existing = incoming
		return
	}
	if strings.TrimSpace(incoming) == "" || scalarEqual(*existing, incoming) {
		return
	}
	appendMergeConflict(conflicts, candidateName, field, *existing, incoming)
}

func mergePointerStringField(existing **string, incoming *string, field, candidateName string, conflicts *[]ValidationIssue) {
	if *existing == nil {
		if incoming != nil {
			value := *incoming
			*existing = &value
		}
		return
	}
	if incoming == nil || scalarEqual(**existing, *incoming) {
		return
	}
	appendMergeConflict(conflicts, candidateName, field, **existing, *incoming)
}

func appendMergeConflict(conflicts *[]ValidationIssue, candidateName, field, first, second string) {
	*conflicts = append(*conflicts, ValidationIssue{
		Code:        "merge_conflict",
		Message:     fmt.Sprintf("candidate %q has conflicting %s values %q and %q", candidateName, field, first, second),
		Recoverable: true,
	})
}

func mergeDescription(first, second string) string {
	if strings.TrimSpace(first) == "" {
		return second
	}
	if strings.TrimSpace(second) == "" {
		return first
	}
	if normalizeWhitespace(first) == normalizeWhitespace(second) {
		return first
	}

	firstWords := strings.Fields(first)
	secondWords := strings.Fields(second)
	maxOverlap := len(firstWords)
	if len(secondWords) < maxOverlap {
		maxOverlap = len(secondWords)
	}
	for overlap := maxOverlap; overlap > 0; overlap-- {
		matches := true
		for i := 0; i < overlap; i++ {
			if !strings.EqualFold(firstWords[len(firstWords)-overlap+i], secondWords[i]) {
				matches = false
				break
			}
		}
		if matches {
			if overlap == len(secondWords) {
				return first
			}
			return joinDescription(first, dropLeadingWords(second, overlap))
		}
	}
	return joinDescription(first, second)
}

func joinDescription(first, second string) string {
	if strings.TrimRightFunc(first, unicode.IsSpace) != first {
		return first + second
	}
	if strings.TrimLeftFunc(second, unicode.IsSpace) != second {
		return first + second
	}
	return first + " " + second
}

func dropLeadingWords(value string, count int) string {
	remaining := value
	for skipped := 0; skipped < count; skipped++ {
		remaining = strings.TrimLeftFunc(remaining, unicode.IsSpace)
		if remaining == "" {
			return ""
		}
		wordEnd := strings.IndexFunc(remaining, unicode.IsSpace)
		if wordEnd < 0 {
			return ""
		}
		remaining = remaining[wordEnd:]
	}
	return remaining
}

func unionPages(first, second []int) []int {
	pages := append(append([]int(nil), first...), second...)
	return uniqueSortedPages(pages)
}

func unionEffects(first, second []RawEffect) []RawEffect {
	result := append([]RawEffect(nil), first...)
	seen := make(map[string]struct{}, len(first)+len(second))
	for _, effect := range first {
		seen[effectKey(effect)] = struct{}{}
	}
	for _, effect := range second {
		key := effectKey(effect)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, effect)
	}
	return result
}

func unionLimitations(first, second []RawLimitation) []RawLimitation {
	result := append([]RawLimitation(nil), first...)
	seen := make(map[string]struct{}, len(first)+len(second))
	for _, limitation := range first {
		seen[limitationKey(limitation)] = struct{}{}
	}
	for _, limitation := range second {
		key := limitationKey(limitation)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, limitation)
	}
	return result
}

func unionStrings(first, second []string) []string {
	result := append([]string(nil), first...)
	seen := make(map[string]struct{}, len(first)+len(second))
	for _, value := range first {
		seen[normalizeWhitespace(value)] = struct{}{}
	}
	for _, value := range second {
		key := normalizeWhitespace(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func effectKey(effect RawEffect) string {
	return normalizeWhitespace(effect.CategoryRaw) + "|" + normalizeWhitespace(effect.Description)
}

func limitationKey(limitation RawLimitation) string {
	index := "nil"
	if limitation.EffectIndex != nil {
		index = strconv.Itoa(*limitation.EffectIndex)
	}
	return index + "|" + normalizeWhitespace(limitation.Description)
}

func pagesTouch(first, second []int) bool {
	for _, firstPage := range first {
		for _, secondPage := range second {
			if abs(firstPage-secondPage) <= 1 {
				return true
			}
		}
	}
	return false
}

func candidatePageStart(candidate RawCandidate) int {
	if len(candidate.SourcePages) == 0 {
		return int(^uint(0) >> 1)
	}
	start := candidate.SourcePages[0]
	for _, page := range candidate.SourcePages[1:] {
		if page < start {
			start = page
		}
	}
	return start
}

func uniqueSortedPages(pages []int) []int {
	result := append([]int(nil), pages...)
	sort.Ints(result)
	if len(result) == 0 {
		return result
	}
	unique := result[:1]
	for _, page := range result[1:] {
		if page != unique[len(unique)-1] {
			unique = append(unique, page)
		}
	}
	return unique
}

func normalizeIdentity(value string) string {
	return strings.TrimFunc(normalizeWhitespace(strings.ToLower(value)), unicode.IsPunct)
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func scalarEqual(first, second string) bool {
	return strings.EqualFold(normalizeWhitespace(first), normalizeWhitespace(second))
}

func cloneCandidate(candidate RawCandidate) RawCandidate {
	clone := candidate
	clone.SourcePages = append([]int(nil), candidate.SourcePages...)
	clone.Effects = append([]RawEffect(nil), candidate.Effects...)
	clone.Limitations = append([]RawLimitation(nil), candidate.Limitations...)
	clone.ReviewReasons = append([]string(nil), candidate.ReviewReasons...)
	if candidate.SourceItemSubtypeRaw != nil {
		value := *candidate.SourceItemSubtypeRaw
		clone.SourceItemSubtypeRaw = &value
	}
	if candidate.WearSlotRaw != nil {
		value := *candidate.WearSlotRaw
		clone.WearSlotRaw = &value
	}
	if candidate.AttunementRequirement != nil {
		value := *candidate.AttunementRequirement
		clone.AttunementRequirement = &value
	}
	return clone
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
