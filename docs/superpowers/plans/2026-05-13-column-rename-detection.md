# Column Rename Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement automatic column rename detection in `sqlx.Diff.askForColumns` so that identical columns with different names produce `RenameColumn` instead of `DropColumn` + `AddColumn`.

**Architecture:** Replace the `askForColumns` stub in `sql/internal/sqlx/diff.go` with logic that: (1) pairs `DropColumn`/`AddColumn` changes, (2) uses `ColumnChange` to check if columns are identical except for name, (3) auto-converts unique pairs to `RenameColumn`, and (4) uses `AskFunc` for ambiguous cases.

**Tech Stack:** Go, `ariga.io/atlas/sql/schema`, `github.com/stretchr/testify`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `sql/internal/sqlx/diff.go` | Modify | Replace `askForColumns` stub with rename detection logic |
| `sql/internal/sqlx/diff_test.go` | Create | Unit tests for rename detection (14 test cases) |

---

### Task 1: Write tests for single exact rename (the happy path)

**Files:**
- Create: `sql/internal/sqlx/diff_test.go`

This task creates the test file with a mock `DiffDriver` and the first test case: a single DropColumn + AddColumn with identical types produces a RenameColumn.

- [ ] **Step 1: Create test file with mock DiffDriver and first test**

Create `sql/internal/sqlx/diff_test.go`:

```go
// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package sqlx

import (
	"fmt"
	"reflect"
	"testing"

	"ariga.io/atlas/sql/schema"

	"github.com/stretchr/testify/require"
)

// mockDiffDriver is a minimal DiffDriver for testing askForColumns.
// Its ColumnChange compares column type, nullability, and default by
// value equality — good enough for unit tests without driver-specific logic.
type mockDiffDriver struct{}

func (mockDiffDriver) RealmObjectDiff(_, _ *schema.Realm) ([]schema.Change, error) {
	return nil, nil
}

func (mockDiffDriver) SchemaAttrDiff(_, _ *schema.Schema) []schema.Change { return nil }

func (mockDiffDriver) SchemaObjectDiff(_, _ *schema.Schema, _ *schema.DiffOptions) ([]schema.Change, error) {
	return nil, nil
}

func (mockDiffDriver) TableAttrDiff(_, _ *schema.Table, _ *schema.DiffOptions) ([]schema.Change, error) {
	return nil, nil
}

func (mockDiffDriver) ColumnChange(_ *schema.Table, from, to *schema.Column, _ *schema.DiffOptions) (schema.Change, error) {
	var change schema.ChangeKind
	if from.Type.Null != to.Type.Null {
		change |= schema.ChangeNull
	}
	if reflect.TypeOf(from.Type.Type) != reflect.TypeOf(to.Type.Type) {
		change |= schema.ChangeType
	} else {
		ft, err := formatSimpleType(from.Type.Type)
		if err != nil {
			return NoChange, err
		}
		tt, err := formatSimpleType(to.Type.Type)
		if err != nil {
			return NoChange, err
		}
		if ft != tt {
			change |= schema.ChangeType
		}
	}
	if !defaultsEqual(from.Default, to.Default) {
		change |= schema.ChangeDefault
	}
	if change == schema.NoChange {
		return NoChange, nil
	}
	return &schema.ModifyColumn{From: from, To: to, Change: change}, nil
}

func (mockDiffDriver) IndexAttrChanged(_, _ []schema.Attr) bool               { return false }
func (mockDiffDriver) IndexPartAttrChanged(_, _ *schema.Index, _ int) bool     { return false }
func (mockDiffDriver) IsGeneratedIndexName(_ *schema.Table, _ *schema.Index) bool { return false }
func (mockDiffDriver) ReferenceChanged(_, _ schema.ReferenceOption) bool       { return false }
func (mockDiffDriver) ForeignKeyAttrChanged(_, _ []schema.Attr) bool           { return false }

func formatSimpleType(t schema.Type) (string, error) {
	switch t := t.(type) {
	case *schema.IntegerType:
		return fmt.Sprintf("int:%s:%v", t.T, t.Unsigned), nil
	case *schema.StringType:
		return fmt.Sprintf("string:%s:%d", t.T, t.Size), nil
	case *schema.BoolType:
		return fmt.Sprintf("bool:%s", t.T), nil
	case *schema.FloatType:
		return fmt.Sprintf("float:%s:%d", t.T, t.Precision), nil
	case *schema.JSONType:
		return fmt.Sprintf("json:%s", t.T), nil
	default:
		return fmt.Sprintf("%T", t), nil
	}
}

func defaultsEqual(d1, d2 schema.Expr) bool {
	if d1 == nil && d2 == nil {
		return true
	}
	if d1 == nil || d2 == nil {
		return false
	}
	switch d1 := d1.(type) {
	case *schema.Literal:
		d2, ok := d2.(*schema.Literal)
		return ok && d1.V == d2.V
	case *schema.RawExpr:
		d2, ok := d2.(*schema.RawExpr)
		return ok && d1.X == d2.X
	default:
		return false
	}
}

func newTestDiff() *Diff {
	return &Diff{DiffDriver: mockDiffDriver{}}
}

func TestAskForColumns_SingleRename(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("old_name", "int")
	addCol := schema.NewIntColumn("new_name", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addCol},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	rename, ok := got[0].(*schema.RenameColumn)
	require.True(t, ok, "expected RenameColumn, got %T", got[0])
	require.Equal(t, "old_name", rename.From.Name)
	require.Equal(t, "new_name", rename.To.Name)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run TestAskForColumns_SingleRename -v`

Expected: FAIL — the current stub returns changes unchanged, so we get 2 changes (DropColumn + AddColumn) instead of 1 (RenameColumn).

- [ ] **Step 3: Commit the failing test**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add failing test for column rename detection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Implement askForColumns rename detection (core logic)

**Files:**
- Modify: `sql/internal/sqlx/diff.go` (replace `askForColumns` stub, currently at line 834 of `sqlx.go`)

Note: the `askForColumns` stub is defined in `sql/internal/sqlx/sqlx.go:834-836`. We need to move it to `diff.go` (where it's called from, line 336) or keep it in `sqlx.go`. Since the method is small and closely related to diffing, we move the implementation to `diff.go` for cohesion and delete the stub from `sqlx.go`.

- [ ] **Step 1: Remove the stub from sqlx.go**

In `sql/internal/sqlx/sqlx.go`, delete lines 834-836:

```go
func (*Diff) askForColumns(_ *schema.Table, changes []schema.Change, _ *schema.DiffOptions) ([]schema.Change, error) {
	return changes, nil // unimplemented.
}
```

- [ ] **Step 2: Add the implementation to diff.go**

Add the following after the `columnDiff` function (after line 343) in `sql/internal/sqlx/diff.go`:

```go
// askForColumns detects column renames by pairing DropColumn and AddColumn
// changes whose columns are identical (per ColumnChange) except for name.
// Unique pairs are auto-converted to RenameColumn. Ambiguous pairs use
// DiffOptions.AskFunc if available, otherwise fall back to drop+add.
func (d *Diff) askForColumns(fromT *schema.Table, changes []schema.Change, opts *schema.DiffOptions) ([]schema.Change, error) {
	var (
		drops   []*schema.DropColumn
		adds    []*schema.AddColumn
		rest    []schema.Change
	)
	for _, c := range changes {
		switch c := c.(type) {
		case *schema.DropColumn:
			drops = append(drops, c)
		case *schema.AddColumn:
			adds = append(adds, c)
		default:
			rest = append(rest, c)
		}
	}
	if len(drops) == 0 || len(adds) == 0 {
		return changes, nil
	}
	// Build candidate matrix: candidates[i] holds indices into adds
	// that are identical to drops[i] (except for name).
	candidates := make([][]int, len(drops))
	for i, dc := range drops {
		for j, ac := range adds {
			match, err := d.columnsMatch(fromT, dc.C, ac.C, opts)
			if err != nil {
				return nil, err
			}
			if match {
				candidates[i] = append(candidates[i], j)
			}
		}
	}
	var (
		renames     []schema.Change
		usedDrops   = make(map[int]bool)
		usedAdds    = make(map[int]bool)
	)
	// Pass 1: resolve unique 1:1 matches.
	for i, cands := range candidates {
		if len(cands) != 1 {
			continue
		}
		j := cands[0]
		// Check reverse uniqueness: add[j] must only match drop[i].
		reverseCount := 0
		for ii, cc := range candidates {
			if ii == i {
				continue
			}
			for _, jj := range cc {
				if jj == j {
					reverseCount++
				}
			}
		}
		if reverseCount > 0 {
			continue
		}
		renames = append(renames, &schema.RenameColumn{
			From: drops[i].C,
			To:   adds[j].C,
		})
		usedDrops[i] = true
		usedAdds[j] = true
	}
	// Pass 2: resolve ambiguous matches via AskFunc.
	for i, cands := range candidates {
		if usedDrops[i] || len(cands) == 0 {
			continue
		}
		// Filter out already-consumed adds.
		var available []int
		for _, j := range cands {
			if !usedAdds[j] {
				available = append(available, j)
			}
		}
		if len(available) == 0 {
			continue
		}
		if len(available) == 1 {
			// After filtering, became unique.
			j := available[0]
			renames = append(renames, &schema.RenameColumn{
				From: drops[i].C,
				To:   adds[j].C,
			})
			usedDrops[i] = true
			usedAdds[j] = true
			continue
		}
		if opts == nil || opts.AskFunc == nil {
			continue
		}
		options := make([]string, 0, len(available)+1)
		for _, j := range available {
			options = append(options, adds[j].C.Name)
		}
		options = append(options, "(none)")
		answer, err := opts.AskFunc(
			fmt.Sprintf("Column %q was removed and columns with identical type were added. Did you mean to rename it?", drops[i].C.Name),
			options,
		)
		if err != nil {
			return nil, err
		}
		if answer == "(none)" {
			continue
		}
		for _, j := range available {
			if adds[j].C.Name == answer {
				renames = append(renames, &schema.RenameColumn{
					From: drops[i].C,
					To:   adds[j].C,
				})
				usedDrops[i] = true
				usedAdds[j] = true
				break
			}
		}
	}
	// Reconstruct: rest + unconsumed drops + renames + unconsumed adds.
	result := make([]schema.Change, 0, len(changes))
	result = append(result, rest...)
	for i, dc := range drops {
		if !usedDrops[i] {
			result = append(result, dc)
		}
	}
	result = append(result, renames...)
	for j, ac := range adds {
		if !usedAdds[j] {
			result = append(result, ac)
		}
	}
	return result, nil
}

// columnsMatch reports whether two columns are identical except for their name.
// It temporarily aligns the names and delegates to ColumnChange.
func (d *Diff) columnsMatch(fromT *schema.Table, from, to *schema.Column, opts *schema.DiffOptions) (bool, error) {
	origName := from.Name
	from.Name = to.Name
	defer func() { from.Name = origName }()
	change, err := d.ColumnChange(fromT, from, to, opts)
	if err != nil {
		return false, err
	}
	return change == NoChange, nil
}
```

- [ ] **Step 3: Add the `fmt` import if not already present in diff.go**

Check the import block at the top of `sql/internal/sqlx/diff.go`. It already imports `"fmt"` (used on line 172), so no changes needed.

- [ ] **Step 4: Run the first test to verify it passes**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run TestAskForColumns_SingleRename -v`

Expected: PASS

- [ ] **Step 5: Commit the implementation**

```bash
git add sql/internal/sqlx/diff.go sql/internal/sqlx/sqlx.go
git commit -m "feat: implement column rename detection in askForColumns

Detect column renames by pairing DropColumn/AddColumn changes
whose columns are identical except for name. Unique pairs are
auto-converted to RenameColumn. Ambiguous pairs use AskFunc.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Add tests for type mismatch (no rename) and nullable/default differences

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add type mismatch test**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_TypeMismatch(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int")
	addCol := schema.NewStringColumn("b", "varchar")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addCol},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.AddColumn{}, got[1])
}

func TestAskForColumns_NullableDiffers(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int") // NOT NULL (default)
	addCol := schema.NewNullIntColumn("b", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addCol},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.AddColumn{}, got[1])
}

func TestAskForColumns_DefaultDiffers(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int").SetDefault(&schema.Literal{V: "0"})
	addCol := schema.NewIntColumn("b", "int").SetDefault(&schema.Literal{V: "1"})
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addCol},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.AddColumn{}, got[1])
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run "TestAskForColumns_(TypeMismatch|NullableDiffers|DefaultDiffers)" -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection tests for type/nullable/default mismatches

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Add tests for multiple independent renames

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add multiple rename test**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_MultipleIndependentRenames(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropA := schema.NewIntColumn("a", "int")
	dropC := schema.NewStringColumn("c", "varchar")
	addB := schema.NewIntColumn("b", "int")
	addD := schema.NewStringColumn("d", "varchar")
	changes := []schema.Change{
		&schema.DropColumn{C: dropA},
		&schema.DropColumn{C: dropC},
		&schema.AddColumn{C: addB},
		&schema.AddColumn{C: addD},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Both should be renames.
	rename1, ok := got[0].(*schema.RenameColumn)
	require.True(t, ok)
	rename2, ok := got[1].(*schema.RenameColumn)
	require.True(t, ok)
	// a->b (int), c->d (varchar)
	require.Equal(t, "a", rename1.From.Name)
	require.Equal(t, "b", rename1.To.Name)
	require.Equal(t, "c", rename2.From.Name)
	require.Equal(t, "d", rename2.To.Name)
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run TestAskForColumns_MultipleIndependentRenames -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection test for multiple independent renames

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Add tests for no drops / no adds edge cases

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add edge case tests**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_NoDrops(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	changes := []schema.Change{
		&schema.AddColumn{C: schema.NewIntColumn("a", "int")},
		&schema.AddColumn{C: schema.NewIntColumn("b", "int")},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Equal(t, changes, got)
}

func TestAskForColumns_NoAdds(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	changes := []schema.Change{
		&schema.DropColumn{C: schema.NewIntColumn("a", "int")},
		&schema.DropColumn{C: schema.NewIntColumn("b", "int")},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Equal(t, changes, got)
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run "TestAskForColumns_(NoDrops|NoAdds)" -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection edge case tests for no drops/adds

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: Add test for mixed changes preserved alongside rename

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add mixed changes test**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_MixedChangesPreserved(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	modifyFrom := schema.NewIntColumn("x", "int")
	modifyTo := schema.NewIntColumn("x", "bigint")
	dropCol := schema.NewStringColumn("a", "varchar")
	addCol := schema.NewStringColumn("b", "varchar")
	changes := []schema.Change{
		&schema.ModifyColumn{From: modifyFrom, To: modifyTo, Change: schema.ChangeType},
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addCol},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	// ModifyColumn preserved first.
	require.IsType(t, &schema.ModifyColumn{}, got[0])
	// Then the rename.
	rename, ok := got[1].(*schema.RenameColumn)
	require.True(t, ok)
	require.Equal(t, "a", rename.From.Name)
	require.Equal(t, "b", rename.To.Name)
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run TestAskForColumns_MixedChangesPreserved -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection test for mixed changes preservation

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Add tests for AskFunc — ambiguous select, none, and error

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add AskFunc tests**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_AmbiguousAskFuncSelects(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int")
	addB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addB},
		&schema.AddColumn{C: addC},
	}
	opts := &schema.DiffOptions{
		AskFunc: func(question string, options []string) (string, error) {
			require.Contains(t, question, `"a"`)
			require.Contains(t, options, "b")
			require.Contains(t, options, "c")
			require.Contains(t, options, "(none)")
			return "b", nil
		},
	}
	got, err := d.askForColumns(tbl, changes, opts)
	require.NoError(t, err)
	require.Len(t, got, 2)
	rename, ok := got[0].(*schema.RenameColumn)
	require.True(t, ok)
	require.Equal(t, "a", rename.From.Name)
	require.Equal(t, "b", rename.To.Name)
	// c remains as AddColumn.
	require.IsType(t, &schema.AddColumn{}, got[1])
	require.Equal(t, "c", got[1].(*schema.AddColumn).C.Name)
}

func TestAskForColumns_AmbiguousNoAskFunc(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int")
	addB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addB},
		&schema.AddColumn{C: addC},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.AddColumn{}, got[1])
	require.IsType(t, &schema.AddColumn{}, got[2])
}

func TestAskForColumns_AmbiguousAskFuncNone(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int")
	addB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addB},
		&schema.AddColumn{C: addC},
	}
	opts := &schema.DiffOptions{
		AskFunc: func(string, []string) (string, error) {
			return "(none)", nil
		},
	}
	got, err := d.askForColumns(tbl, changes, opts)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.AddColumn{}, got[1])
	require.IsType(t, &schema.AddColumn{}, got[2])
}

func TestAskForColumns_AskFuncError(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropCol := schema.NewIntColumn("a", "int")
	addB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropCol},
		&schema.AddColumn{C: addB},
		&schema.AddColumn{C: addC},
	}
	opts := &schema.DiffOptions{
		AskFunc: func(string, []string) (string, error) {
			return "", fmt.Errorf("user cancelled")
		},
	}
	_, err := d.askForColumns(tbl, changes, opts)
	require.EqualError(t, err, "user cancelled")
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run "TestAskForColumns_Ambiguous|TestAskForColumns_AskFunc" -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection tests for AskFunc ambiguous cases

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Add tests for reverse ambiguity (multiple drops, one add)

**Files:**
- Modify: `sql/internal/sqlx/diff_test.go`

- [ ] **Step 1: Add reverse ambiguity tests**

Append to `sql/internal/sqlx/diff_test.go`:

```go
func TestAskForColumns_ReverseAmbiguousAskFunc(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropA := schema.NewIntColumn("a", "int")
	dropB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropA},
		&schema.DropColumn{C: dropB},
		&schema.AddColumn{C: addC},
	}
	opts := &schema.DiffOptions{
		AskFunc: func(question string, options []string) (string, error) {
			// Both a and b match c. AskFunc is called for "a" first.
			// Select "c" for "a".
			if question != "" {
				return "c", nil
			}
			return "(none)", nil
		},
	}
	got, err := d.askForColumns(tbl, changes, opts)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// drop[b] remains as DropColumn.
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.Equal(t, "b", got[0].(*schema.DropColumn).C.Name)
	// a -> c rename.
	rename, ok := got[1].(*schema.RenameColumn)
	require.True(t, ok)
	require.Equal(t, "a", rename.From.Name)
	require.Equal(t, "c", rename.To.Name)
}

func TestAskForColumns_ReverseAmbiguousNoAskFunc(t *testing.T) {
	d := newTestDiff()
	tbl := schema.NewTable("users").SetSchema(schema.New("public"))
	dropA := schema.NewIntColumn("a", "int")
	dropB := schema.NewIntColumn("b", "int")
	addC := schema.NewIntColumn("c", "int")
	changes := []schema.Change{
		&schema.DropColumn{C: dropA},
		&schema.DropColumn{C: dropB},
		&schema.AddColumn{C: addC},
	}
	got, err := d.askForColumns(tbl, changes, &schema.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.IsType(t, &schema.DropColumn{}, got[1])
	require.IsType(t, &schema.AddColumn{}, got[2])
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -run "TestAskForColumns_ReverseAmbiguous" -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add sql/internal/sqlx/diff_test.go
git commit -m "test: add rename detection tests for reverse ambiguity

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 9: Run full test suite to verify no regressions

**Files:** None (verification only)

- [ ] **Step 1: Run all sqlx tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/internal/sqlx/ -v`

Expected: All tests PASS.

- [ ] **Step 2: Run MySQL diff tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/mysql/ -run TestDiff -v`

Expected: All tests PASS. MySQL's `ColumnChange` is called through the shared `askForColumns` now, and existing behavior (no renames in current tests) is preserved.

- [ ] **Step 3: Run Postgres diff tests**

Run: `cd /Users/william_w_chen/Desktop/atlas && go test ./sql/postgres/ -run TestDiff -v`

Expected: All tests PASS.

- [ ] **Step 4: Run full project build**

Run: `cd /Users/william_w_chen/Desktop/atlas && go build ./...`

Expected: Build succeeds with no errors.
