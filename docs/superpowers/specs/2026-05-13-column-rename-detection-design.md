# Column Rename Detection Design

## Overview

Implement automatic column rename detection in the shared `sqlx.Diff` layer so that when a column is renamed (name changes but type/nullable/default remain identical), the differ produces a `RenameColumn` change instead of `DropColumn` + `AddColumn`.

## Problem

Currently, the differ matches columns by exact name. If a column is renamed:
- `from: users(first_name varchar(255))` → `to: users(given_name varchar(255))`
- Result: `DropColumn(first_name)` + `AddColumn(given_name)` — data loss on migration.

The desired behavior: `RenameColumn(first_name → given_name)` — preserves data.

## Scope

- **In scope:** Column rename detection only.
- **Out of scope:** Table rename, index rename, rename+modify (column renamed AND type changed simultaneously).
- **Future work:** Table and index rename detection can follow the same pattern using `fixRenames` and `askForIndexes`.

## Design

### Implementation Location

`sql/internal/sqlx/diff.go` — modify the existing `askForColumns` stub method on `*Diff`.

This method is already called at the right point in `columnDiff()` (line 336), receives the full change list and `*schema.DiffOptions` (which includes `AskFunc`), and is designed as an extension point for exactly this purpose.

### Algorithm

```
askForColumns(fromT *Table, changes []Change, opts *DiffOptions) ([]Change, error):

1. Separate changes into: drops []DropColumn, adds []AddColumn, rest []Change
   - If len(drops) == 0 || len(adds) == 0: return changes unchanged

2. Build candidate matrix:
   - For each (drop[i], add[j]) pair:
     - Temporarily set drop[i].C.Name = add[j].C.Name
     - Call d.ColumnChange(fromT, drop[i].C, add[j].C, opts)
     - If result == NoChange → mark as candidate
     - Restore original name

3. Resolve unique matches (auto-detect):
   - For each drop[i] with exactly 1 candidate add[j],
     AND add[j] has exactly 1 candidate drop[i]:
     → Replace with RenameColumn{From: drop[i].C, To: add[j].C}
     → Mark both as consumed

4. Resolve ambiguous matches (AskFunc):
   - For each unconsumed drop[i] with multiple candidate adds:
     - If opts.AskFunc != nil:
       - Call AskFunc with question and candidate names + "(none)" option
       - If user selects a candidate → RenameColumn, mark both consumed
       - If user selects "(none)" → keep as drop+add
       - If AskFunc returns error → propagate error
     - If opts.AskFunc == nil:
       - Keep as drop+add (safe default)

5. Reconstruct change list:
   - Append: rest + unconsumed drops + renames + unconsumed adds
   - Preserve relative order where possible
```

### Column Similarity via ColumnChange

The `ColumnChange` method on `DiffDriver` is already implemented by each driver (MySQL, Postgres, SQLite) with full type-comparison logic including driver-specific nuances (charset, collation, generated expressions, etc.). By reusing it, we get accurate similarity detection for free without duplicating any driver logic.

A match requires `ColumnChange` to return `NoChange` — meaning the columns are identical in every respect except name.

### AskFunc Integration

The existing `DiffOptions.AskFunc` signature:
```go
AskFunc func(string, []string) (string, error)
```

For ambiguous renames:
- Question: `Column "old_name" was removed and columns with the same type were added. Did you mean to rename it?`
- Options: `["new_name1", "new_name2", "(none)"]`
- Return value: the selected option name, or "(none)" to skip.

### Safety Guarantees

1. **No false positives on unique match:** Only auto-detects when there is exactly one DropColumn that matches exactly one AddColumn, and vice versa.
2. **Safe fallback:** When ambiguous and no AskFunc, falls back to drop+add (current behavior).
3. **No data loss:** `RenameColumn` preserves column data; the old drop+add approach deletes it.
4. **Driver-agnostic:** Works for all drivers through the shared `ColumnChange` interface.

## Test Plan

### Unit Tests (in `sql/internal/sqlx/diff_test.go`)

| # | Scenario | Input | Expected Output |
|---|---|---|---|
| 1 | Single exact rename | DropCol(a, int) + AddCol(b, int) | RenameColumn(a→b) |
| 2 | Type mismatch, no rename | DropCol(a, int) + AddCol(b, varchar) | DropCol(a) + AddCol(b) |
| 3 | Multiple independent renames | DropCol(a, int) + DropCol(c, varchar) + AddCol(b, int) + AddCol(d, varchar) | RenameCol(a→b) + RenameCol(c→d) |
| 4 | Ambiguous, AskFunc selects | DropCol(a, int) + AddCol(b, int) + AddCol(c, int), AskFunc→"b" | RenameCol(a→b) + AddCol(c) |
| 5 | Ambiguous, AskFunc=nil | DropCol(a, int) + AddCol(b, int) + AddCol(c, int) | DropCol(a) + AddCol(b) + AddCol(c) |
| 6 | Ambiguous, AskFunc says none | DropCol(a, int) + AddCol(b, int) + AddCol(c, int), AskFunc→"(none)" | DropCol(a) + AddCol(b) + AddCol(c) |
| 7 | No drops | AddCol(a) + AddCol(b) | AddCol(a) + AddCol(b) |
| 8 | No adds | DropCol(a) + DropCol(b) | DropCol(a) + DropCol(b) |
| 9 | Mixed changes preserved | ModifyCol(x) + DropCol(a) + AddCol(b, same type) | ModifyCol(x) + RenameCol(a→b) |
| 10 | Nullable differs, no rename | DropCol(a, int NOT NULL) + AddCol(b, int NULL) | DropCol(a) + AddCol(b) |
| 11 | Default differs, no rename | DropCol(a, int DEFAULT 0) + AddCol(b, int DEFAULT 1) | DropCol(a) + AddCol(b) |
| 12 | AskFunc returns error | DropCol(a) + AddCol(b) + AddCol(c), AskFunc→error | error propagated |
| 13 | Multiple drops, one add (reverse ambiguity) | DropCol(a, int) + DropCol(b, int) + AddCol(c, int), AskFunc→"a" | RenameCol(a→c) + DropCol(b) |
| 14 | Multiple drops, one add, AskFunc=nil | DropCol(a, int) + DropCol(b, int) + AddCol(c, int) | DropCol(a) + DropCol(b) + AddCol(c) |

### Integration Tests

- Test through MySQL `SchemaDiff` to verify end-to-end rename detection with real MySQL column types.
- Verify that the generated SQL uses `RENAME COLUMN` (MySQL 8+) or `CHANGE COLUMN` (older versions).

## Files to Modify

1. `sql/internal/sqlx/diff.go` — implement `askForColumns` (replace stub)
2. `sql/internal/sqlx/diff_test.go` — add unit tests for rename detection

## Files NOT Modified

- `sql/schema/migrate.go` — no changes needed, types already exist
- `sql/mysql/diff.go` — no changes needed, `ColumnChange` already works
- `sql/mysql/migrate.go` — already handles `RenameColumn`
- `sql/postgres/diff.go` — no changes needed
- `sql/postgres/migrate.go` — already handles `RenameColumn`
