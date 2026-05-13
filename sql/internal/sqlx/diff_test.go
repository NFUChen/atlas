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

func (mockDiffDriver) IndexAttrChanged(_, _ []schema.Attr) bool                  { return false }
func (mockDiffDriver) IndexPartAttrChanged(_, _ *schema.Index, _ int) bool        { return false }
func (mockDiffDriver) IsGeneratedIndexName(_ *schema.Table, _ *schema.Index) bool { return false }
func (mockDiffDriver) ReferenceChanged(_, _ schema.ReferenceOption) bool          { return false }
func (mockDiffDriver) ForeignKeyAttrChanged(_, _ []schema.Attr) bool              { return false }

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
	dropCol := schema.NewIntColumn("a", "int")
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
	rename1, ok := got[0].(*schema.RenameColumn)
	require.True(t, ok)
	rename2, ok := got[1].(*schema.RenameColumn)
	require.True(t, ok)
	require.Equal(t, "a", rename1.From.Name)
	require.Equal(t, "b", rename1.To.Name)
	require.Equal(t, "c", rename2.From.Name)
	require.Equal(t, "d", rename2.To.Name)
}

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
	require.IsType(t, &schema.ModifyColumn{}, got[0])
	rename, ok := got[1].(*schema.RenameColumn)
	require.True(t, ok)
	require.Equal(t, "a", rename.From.Name)
	require.Equal(t, "b", rename.To.Name)
}

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
			return "c", nil
		},
	}
	got, err := d.askForColumns(tbl, changes, opts)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.IsType(t, &schema.DropColumn{}, got[0])
	require.Equal(t, "b", got[0].(*schema.DropColumn).C.Name)
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
