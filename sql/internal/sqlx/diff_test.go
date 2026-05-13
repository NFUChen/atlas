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
