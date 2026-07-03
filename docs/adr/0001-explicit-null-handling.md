# 1. Explicit-null handling in GraphQL variables

Date: 2026-07-02

## Status

Accepted. The two defects below are fixed; retiring the sentinel for
genqlient-native null handling remains a longer-term follow-up (see "Why not
genqlient-native today").

## Context

Linear mutations distinguish three states for an optional input field:

- **Absent** - the field is left unchanged.
- **Explicit null** - the field is cleared (for example, `issue update --project ""` removes the project).
- **A value** - the field is set.

Linear's API also rejects explicit `null` on many inputs where the caller meant
"absent", so the client cannot simply send `null` for every unset field.

genqlient generates most optional input fields as plain Go types with
`json:"field"` (no `omitempty`), so a zero value still serializes and the
generated types alone cannot express all three states. lincli needs a way to
send an explicit null on demand while omitting the fields the caller never set.

## Decision

`pkg/api/client.go` defines a sentinel string, `NullSentinel`
(`"__LINCLI_NULL__"`), and a `stripNulls` pass that runs on every request:

- A field whose value is Go `nil` is dropped, so unset fields are absent.
- A field whose string value equals `NullSentinel` is rewritten to JSON `null`,
  so a command can clear a field by setting it to the sentinel.

Commands that support clearing a field (for example `issue update`,
`project update`, `initiative update`) assign `api.NullSentinel` when the user
passes an empty value.

## Consequences

The sentinel keeps the command code simple and needs no changes to the
generated types. It originally carried two defects, both now fixed at the
`stripNulls`/`NullSentinel` layer:

1. **Empty objects were dropped (fixed).** `stripNulls` used to discard a nested
   map once it stripped to empty, so an intentionally empty object never reached
   the API. `stripNulls` now preserves empty objects, matching how it already
   handled empty maps inside slices; only Go `nil` values (genqlient's
   serialization of unset optional fields) are removed.
2. **A colliding literal was silently nulled (fixed).** The sentinel used to be
   the fixed string `"__LINCLI_NULL__"`, so any free-text value equal to it (an
   issue title, a comment body) was rewritten to null. `NullSentinel` is now a
   per-process value with 16 random bytes appended, so no user-supplied value
   can collide with it.

### Why not genqlient-native today

The cleaner end state is to retire the sentinel and let genqlient express the
three states directly, then delete `stripNulls`, `NullSentinel`, and the
per-command sentinel assignments. genqlient v0.8.1 blocks this: it emits
`omitempty` on filter and comparator input fields but not on mutation-input
fields (about 2100 pointer fields carry no `omitempty`), and it offers no
directive to add it. Without `omitempty`, a nil pointer still serializes as
`null`, so `stripNulls` is still needed to distinguish "absent" from "clear".

Revisit when genqlient can emit `omitempty` on mutation inputs (or when a
post-generation step is deemed acceptable). At that point:

- A nil pointer omits the field, so `stripNulls`'s null-dropping is unnecessary.
- Explicit null moves onto genqlient's typed null handling, so the sentinel and
  its string substitution can be deleted.

## Alternatives considered

- **Send `null` for every unset field.** Rejected: Linear rejects explicit null
  on inputs where the caller meant "absent".
- **A wrapper type per nullable field.** Rejected for now as more code than the
  sentinel, though the eventual genqlient-native handling is a typed variant of
  this.
