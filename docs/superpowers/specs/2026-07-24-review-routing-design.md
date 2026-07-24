# Review Routing Design

## Decision

Extraction uncertainty is routed by a structured candidate field rather than
by matching free-form review text:

- `none`: no model-reported uncertainty.
- `visual_ambiguity`: a title, number, closed-vocabulary value, or similarly
  factual detail cannot be determined from OCR text and the source page image
  could resolve it.
- `source_ambiguity`: the supplied source is incomplete, narrative-only, or
  semantically ambiguous; another image request cannot reliably resolve it.

`merge_conflict` is a locally generated issue. It never triggers image
recovery. It marks an otherwise-normalized candidate for human review with
the conflict evidence preserved.

## Routing

1. Normalize and validate each merged candidate.
2. If only merge conflicts remain, add their messages to review reasons and
   retain the candidate in review without an API call.
3. If a normalization/validation issue remains, use at most one image recovery
   request, as today.
4. If the candidate has `visual_ambiguity` but no validation issue, make one
   image recovery request. The recovery prompt must return `none` when the
   image resolves the detail; unresolved ambiguity stays in review rather than
   retrying indefinitely.
5. `source_ambiguity` stays in review and never makes an image request.

## Scope

The model schema and prompt gain `review_kind`. Existing free-form
`review_reasons` remain evidence. Tests cover routing decisions without an API
call. A later paid full dry run is the only evidence that the model reliably
assigns `review_kind` on the real PDF.
