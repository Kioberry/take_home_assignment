package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCandidateKeyNormalizesIdentityPagesAndDescription(t *testing.T) {
	first := RawCandidate{
		Name:           "  Exo-Armor! ",
		SourcePages:    []int{9, 7, 8},
		RawDescription: "  Protects\n\t the wearer. ",
	}
	second := RawCandidate{
		Name:           "exo-armor",
		SourcePages:    []int{7, 8, 9},
		RawDescription: "Protects the wearer.",
	}

	if got, want := CandidateKey(first), CandidateKey(second); got != want {
		t.Fatalf("CandidateKey values differ: %q vs %q", got, want)
	}
}

func TestMergeCandidatesRemovesExactOverlapDuplicate(t *testing.T) {
	candidate := mergeCandidate("Arcane Compass", []int{7}, "Points north.", false)
	merged, conflicts := MergeCandidates([][]RawCandidate{{candidate}, {candidate}})

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one", len(merged))
	}
	if !reflect.DeepEqual(merged[0], candidate) {
		t.Fatalf("merged candidate = %#v, want %#v", merged[0], candidate)
	}
}

func TestMergeCandidatesMergesSamePageAlternateDescriptions(t *testing.T) {
	first := mergeCandidate("Arcane Compass", []int{5}, "Points north.", false)
	second := mergeCandidate("arcane compass", []int{5}, "Wondrous item. Points north.", false)

	merged, conflicts := MergeCandidates([][]RawCandidate{{first}, {second}})

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one", len(merged))
	}
	if got, want := merged[0].RawDescription, "Wondrous item. Points north."; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestMergeCandidatesMergesCaseOnlyNameVariantsOnTouchingPages(t *testing.T) {
	first := mergeCandidate("DARKSTAR MACE", []int{13}, "A dark mace.", false)
	second := mergeCandidate("Darkstar Mace", []int{13}, "Wondrous item. A dark mace.", false)

	merged, _ := MergeCandidates([][]RawCandidate{{first}, {second}})

	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one", len(merged))
	}
}

func TestMergeCandidatesRetainsSameNameOnDifferentPages(t *testing.T) {
	first := mergeCandidate("Mirror Ring", []int{4}, "Shows a reflection.", false)
	second := mergeCandidate(" mirror   ring ", []int{12}, "Shows another reflection.", false)

	merged, conflicts := MergeCandidates([][]RawCandidate{{first}, {second}})

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != 2 {
		t.Fatalf("merged %d candidates, want two", len(merged))
	}
	if !reflect.DeepEqual(merged[0].SourcePages, []int{4}) || !reflect.DeepEqual(merged[1].SourcePages, []int{12}) {
		t.Fatalf("merged pages = %#v, want [[4] [12]]", []RawCandidate{merged[0], merged[1]})
	}
}

func TestMergeCandidatesMergesAdjacentContinuationFragmentsInPageOrder(t *testing.T) {
	ending := mergeCandidate("Chronicle Cloak", []int{9}, "Then records the final event.", true)
	beginning := mergeCandidate("chronicle cloak", []int{8}, "First records the opening event.", false)

	merged, conflicts := MergeCandidates([][]RawCandidate{{ending}, {beginning}})

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one", len(merged))
	}
	wantPages := []int{8, 9}
	if !reflect.DeepEqual(merged[0].SourcePages, wantPages) {
		t.Fatalf("pages = %#v, want %#v", merged[0].SourcePages, wantPages)
	}
	if got, want := merged[0].RawDescription, "First records the opening event. Then records the final event."; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
	if merged[0].Continuation {
		t.Fatal("merged complete candidate remains marked as continuation")
	}
}

func TestMergeCandidatesRetainsEffectsAndLimitationsOnce(t *testing.T) {
	effect := RawEffect{CategoryRaw: "defense", Description: "Raises the bearer’s guard."}
	newEffect := RawEffect{CategoryRaw: "utility", Description: "Reveals hidden doors."}
	effectIndex := 0
	limitation := RawLimitation{EffectIndex: &effectIndex, Description: "Only works at dusk."}
	newLimitation := RawLimitation{Description: "Requires a closed door."}

	first := mergeCandidate("Dusk Ward", []int{15, 16}, "Raises the bearer’s guard.", false)
	first.Effects = []RawEffect{effect}
	first.Limitations = []RawLimitation{limitation}
	first.Confidence = 0.72
	first.ReviewReasons = []string{"ocr ambiguity"}
	second := mergeCandidate("Dusk Ward", []int{16, 17}, "Reveals hidden doors.", true)
	second.Effects = []RawEffect{effect, newEffect}
	second.Limitations = []RawLimitation{limitation, newLimitation}
	second.ReviewReasons = []string{"ocr ambiguity", "continuation"}

	merged, conflicts := MergeCandidates([][]RawCandidate{{first}, {second}})

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one", len(merged))
	}
	if !reflect.DeepEqual(merged[0].Effects, []RawEffect{effect, newEffect}) {
		t.Fatalf("effects = %#v, want %#v", merged[0].Effects, []RawEffect{effect, newEffect})
	}
	if len(merged[0].Limitations) != 2 || !reflect.DeepEqual(merged[0].Limitations[0], limitation) || !reflect.DeepEqual(merged[0].Limitations[1], newLimitation) {
		t.Fatalf("limitations = %#v, want stable union", merged[0].Limitations)
	}
	if merged[0].Confidence != 0.72 || !reflect.DeepEqual(merged[0].ReviewReasons, []string{"ocr ambiguity", "continuation"}) {
		t.Fatalf("confidence/review reasons = %v/%#v, want minimum and stable union", merged[0].Confidence, merged[0].ReviewReasons)
	}
}

func TestMergeCandidatesSurfacesConflictingRarity(t *testing.T) {
	first := mergeCandidate("Conflict Charm", []int{18}, "The first account.", false)
	first.RarityRaw = "rare"
	second := mergeCandidate("Conflict Charm", []int{19}, "The continued account.", true)
	second.RarityRaw = "legendary"

	merged, conflicts := MergeCandidates([][]RawCandidate{{first}, {second}})

	if len(merged) != 1 {
		t.Fatalf("merged %d candidates, want one despite recoverable conflict", len(merged))
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %#v, want one", conflicts)
	}
	issue := conflicts[0]
	if issue.Code != "merge_conflict" || !issue.Recoverable || !strings.Contains(issue.Message, "rarity_raw") {
		t.Fatalf("conflict = %#v, want recoverable rarity_raw conflict", issue)
	}
	if merged[0].RarityRaw != first.RarityRaw {
		t.Fatalf("rarity_raw = %q, want deterministic source-preserving first value %q", merged[0].RarityRaw, first.RarityRaw)
	}
}

func TestMergeCandidatesKnownCrossPageRecords(t *testing.T) {
	cases := []struct {
		name       string
		start, end int
	}{
		{name: "Exo-Armor", start: 7, end: 9},
		{name: "Ring of Elven Lords", start: 22, end: 24},
		{name: "War Drum of the Horde", start: 29, end: 31},
		{name: "Amulet of Encasement", start: 38, end: 39},
	}

	var batches [][]RawCandidate
	for _, tc := range cases {
		batches = append(batches,
			[]RawCandidate{mergeCandidate(tc.name, []int{tc.start}, "Opening text.", false)},
			[]RawCandidate{mergeCandidate(tc.name, []int{tc.start + 1, tc.end}, "Continuation text.", true)},
		)
	}

	merged, conflicts := MergeCandidates(batches)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if len(merged) != len(cases) {
		t.Fatalf("merged %d candidates, want %d", len(merged), len(cases))
	}
	for _, tc := range cases {
		var got RawCandidate
		for _, candidate := range merged {
			if strings.EqualFold(candidate.Name, tc.name) {
				got = candidate
				break
			}
		}
		if got.Name == "" {
			t.Fatalf("missing known cross-page candidate %q", tc.name)
		}
		wantPages := make([]int, 0, tc.end-tc.start+1)
		for page := tc.start; page <= tc.end; page++ {
			wantPages = append(wantPages, page)
		}
		if !reflect.DeepEqual(got.SourcePages, wantPages) {
			t.Errorf("%s pages = %#v, want %#v", tc.name, got.SourcePages, wantPages)
		}
	}
}

func mergeCandidate(name string, pages []int, description string, continuation bool) RawCandidate {
	return RawCandidate{
		Name:              name,
		SourcePages:       append([]int(nil), pages...),
		SourceItemTypeRaw: "wondrous item",
		RarityRaw:         "uncommon",
		UsageModeRaw:      "passive",
		RawDescription:    description,
		Confidence:        0.9,
		Continuation:      continuation,
	}
}

func TestMergeCandidatesOutputIsDeterministic(t *testing.T) {
	first := mergeCandidate("North Star", []int{3}, "Points north.", false)
	second := mergeCandidate("South Star", []int{2}, "Points south.", false)

	one, oneConflicts := MergeCandidates([][]RawCandidate{{first, second}})
	two, twoConflicts := MergeCandidates([][]RawCandidate{{second}, {first}})

	sort.Slice(two, func(i, j int) bool { return CandidateKey(two[i]) < CandidateKey(two[j]) })
	sort.Slice(one, func(i, j int) bool { return CandidateKey(one[i]) < CandidateKey(one[j]) })
	if !reflect.DeepEqual(one, two) || !reflect.DeepEqual(oneConflicts, twoConflicts) {
		t.Fatalf("repeated merge differs: first=%#v/%#v second=%#v/%#v", one, oneConflicts, two, twoConflicts)
	}
}

func TestMergeCandidatesIsPermutationInvariantForSameEarliestPage(t *testing.T) {
	first := mergeCandidate("Shared Relic", []int{7}, "Alpha source text.", false)
	first.SourceItemSubtypeRaw = stringPointer("alpha subtype")
	first.RarityRaw = "rare"
	first.Effects = []RawEffect{{CategoryRaw: "alpha", Description: "Alpha effect."}}
	first.ReviewReasons = []string{"alpha reason"}
	second := mergeCandidate("Shared Relic", []int{7}, "Beta source text.", true)
	second.SourceItemSubtypeRaw = stringPointer("beta subtype")
	second.RarityRaw = "legendary"
	second.Effects = []RawEffect{{CategoryRaw: "beta", Description: "Beta effect."}}
	second.ReviewReasons = []string{"beta reason"}

	forwardCandidates, forwardConflicts := MergeCandidates([][]RawCandidate{{first, second}})
	reversedCandidates, reversedConflicts := MergeCandidates([][]RawCandidate{{second, first}})

	if !reflect.DeepEqual(forwardCandidates, reversedCandidates) {
		t.Fatalf("merged candidates depend on input permutation:\nforward=%#v\nreversed=%#v", forwardCandidates, reversedCandidates)
	}
	if !reflect.DeepEqual(forwardConflicts, reversedConflicts) {
		t.Fatalf("conflicts depend on input permutation:\nforward=%#v\nreversed=%#v", forwardConflicts, reversedConflicts)
	}
	forwardBytes, err := json.Marshal(struct {
		Candidates []RawCandidate
		Conflicts  []ValidationIssue
	}{forwardCandidates, forwardConflicts})
	if err != nil {
		t.Fatalf("marshal forward result: %v", err)
	}
	reversedBytes, err := json.Marshal(struct {
		Candidates []RawCandidate
		Conflicts  []ValidationIssue
	}{reversedCandidates, reversedConflicts})
	if err != nil {
		t.Fatalf("marshal reversed result: %v", err)
	}
	if !bytes.Equal(forwardBytes, reversedBytes) {
		t.Fatalf("serialized result depends on input permutation:\nforward=%s\nreversed=%s", forwardBytes, reversedBytes)
	}
}

func stringPointer(value string) *string {
	return &value
}
