// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package sqlx

import (
	"errors"
	"strconv"
	"testing"

	"ariga.io/atlas/sql/schema"

	"github.com/stretchr/testify/require"
)

func TestModeInspectRealm(t *testing.T) {
	m := ModeInspectRealm(nil)
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))

	m = ModeInspectRealm(&schema.InspectRealmOption{})
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))

	m = ModeInspectRealm(&schema.InspectRealmOption{Mode: schema.InspectSchemas})
	require.True(t, m.Is(schema.InspectSchemas))
	require.False(t, m.Is(schema.InspectTables))

	m = ModeInspectRealm(&schema.InspectRealmOption{Exclude: []string{"*.*"}})
	require.Equal(t, schema.InspectSchemas, m)
	m = ModeInspectRealm(&schema.InspectRealmOption{Exclude: []string{"*"}})
	require.NotEqual(t, schema.InspectSchemas, m)
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))
	m = ModeInspectRealm(&schema.InspectRealmOption{Mode: schema.InspectFuncs, Exclude: []string{"*.*"}})
	require.NotEqual(t, schema.InspectSchemas, m)
	require.True(t, m.Is(schema.InspectFuncs))
	require.False(t, m.Is(schema.InspectTables))
}

func TestModeInspectSchema(t *testing.T) {
	m := ModeInspectSchema(nil)
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))

	m = ModeInspectSchema(&schema.InspectOptions{})
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))

	m = ModeInspectSchema(&schema.InspectOptions{Mode: schema.InspectSchemas})
	require.True(t, m.Is(schema.InspectSchemas))
	require.False(t, m.Is(schema.InspectTables))

	m = ModeInspectSchema(&schema.InspectOptions{Exclude: []string{"*"}})
	require.Equal(t, schema.InspectSchemas, m)
	m = ModeInspectSchema(&schema.InspectOptions{Exclude: []string{"*.*"}})
	require.NotEqual(t, schema.InspectSchemas, m)
	require.True(t, m.Is(schema.InspectSchemas))
	require.True(t, m.Is(schema.InspectTables))
	m = ModeInspectSchema(&schema.InspectOptions{Mode: schema.InspectFuncs, Exclude: []string{"*"}})
	require.NotEqual(t, schema.InspectSchemas, m)
	require.True(t, m.Is(schema.InspectFuncs))
	require.False(t, m.Is(schema.InspectTables))
}

func TestBuilder(t *testing.T) {
	var (
		b       = &Builder{QuoteOpening: '"', QuoteClosing: '"'}
		columns = []string{"a", "b", "c"}
	)
	b.P("CREATE TABLE").
		Table(&schema.Table{Name: "users"}).
		Wrap(func(b *Builder) {
			b.MapComma(columns, func(i int, b *Builder) {
				b.Ident(columns[i]).P("int").P("NOT NULL")
			})
			b.Comma().P("PRIMARY KEY").Wrap(func(b *Builder) {
				b.MapComma(columns, func(i int, b *Builder) {
					b.Ident(columns[i])
				})
			})
		})
	require.Equal(t, `CREATE TABLE "users" ("a" int NOT NULL, "b" int NOT NULL, "c" int NOT NULL, PRIMARY KEY ("a", "b", "c"))`, b.String())

	// WrapErr.
	require.EqualError(t, b.WrapErr(func(*Builder) error { return errors.New("oops") }), "oops")
	require.EqualError(t, b.WrapIndentErr(func(*Builder) error { return errors.New("oops") }), "oops")
}

func TestBuilder_Qualifier(t *testing.T) {
	var (
		s = "other"
		b = &Builder{QuoteOpening: '"', QuoteClosing: '"', Schema: &s}
	)
	b.P("CREATE TABLE").Table(schema.NewTable("users"))
	require.Equal(t, `CREATE TABLE "other"."users"`, b.String())

	// Bypass table schema.
	b.Reset()
	b.P("CREATE TABLE").Table(schema.NewTable("users").SetSchema(schema.New("test")))
	require.Equal(t, `CREATE TABLE "other"."users"`, b.String())

	// Empty qualifier, means skip.
	s = ""
	b.Reset()
	b.P("CREATE TABLE").Table(schema.NewTable("users").SetSchema(schema.New("test")))
	require.Equal(t, `CREATE TABLE "users"`, b.String())
}

func TestQuote(t *testing.T) {
	var (
		s = "s1"
		b = &Builder{QuoteOpening: '[', QuoteClosing: ']', Schema: &s}
	)
	b.P("EXECUTE sp_rename").
		P("@newname = N'c2'").Comma().
		P("@objtype = N'COLUMN'").Comma()
	b.P("@objname = ").Quote("N", func(b *Builder) {
		b.TableResource(schema.NewTable("t1"), &schema.Column{Name: "c1"})
	})
	require.Equal(t, `EXECUTE sp_rename @newname = N'c2', @objtype = N'COLUMN', @objname = N'[s1].[t1].[c1]'`, b.String())
}
func TestMayWrap(t *testing.T) {
	tests := []struct {
		input   string
		wrapped bool
	}{
		{"", true},
		{"()", false},
		{"('text')", false},
		{"('(')", false},
		{`('(\\')`, false},
		{`('\')(')`, false},
		{`(a) in (b)`, true},
		{`a in (b)`, true},
		{`("\\\\(((('")`, false},
		{`('(')||(')')`, true},
		// Test examples from SQLite.
		{"b || 'x'", true},
		{"a+1", true},
		{"substr(x, 2)", true},
		{"(json_extract(x, '$.a'))", false},
		{"(substr(a, 2) COLLATE NOCASE)", false},
		{"(b+random())", false},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			expect := tt.input
			if tt.wrapped {
				expect = "(" + expect + ")"
			}
			require.Equal(t, expect, MayWrap(tt.input))
		})
	}
}

func TestNormalizeCheckExpr(t *testing.T) {
	tests := []struct {
		name string
		from string // Expression from Postgres (ANY(ARRAY[...]) form)
		to   string // Expression from HCL (IN form)
	}{
		{
			name: "basic ANY ARRAY to IN",
			from: `((type)::text = ANY ((ARRAY['A'::character varying, 'B'::character varying])::text[]))`,
			to:   `(type) IN ('A', 'B')`,
		},
		{
			name: "payment_audit_records status check",
			from: `((status)::text = ANY ((ARRAY['PENDING'::character varying, 'SUCCESS'::character varying, 'FAILED'::character varying])::text[]))`,
			to:   `(status) IN ('PENDING', 'SUCCESS', 'FAILED')`,
		},
		{
			name: "lowercase in without column parens",
			from: `((type)::text = ANY ((ARRAY['STRING'::character varying, 'NUMBER'::character varying, 'BOOLEAN'::character varying, 'ARRAY_FLOAT'::character varying, 'ARRAY_STRING'::character varying, 'URI'::character varying, 'EMAIL'::character varying])::text[]))`,
			to:   `type in ('STRING', 'NUMBER', 'BOOLEAN', 'ARRAY_FLOAT', 'ARRAY_STRING', 'URI', 'EMAIL')`,
		},
		{
			name: "features type check",
			from: `((type)::text = ANY ((ARRAY['SOFTWARE'::character varying, 'HARDWARE'::character varying, 'HYBRID'::character varying])::text[]))`,
			to:   `type in ('SOFTWARE', 'HARDWARE', 'HYBRID')`,
		},
		{
			name: "organization_subscriptions status check",
			from: `((status)::text = ANY ((ARRAY['WAITING_FOR_SUBSCRIPTION_APPROVAL'::character varying, 'WAITING_FOR_DEPLOYMENT_APPROVAL'::character varying, 'DEPLOYMENT_APPROVED'::character varying, 'EXPIRED'::character varying])::text[]))`,
			to:   `status in ('WAITING_FOR_SUBSCRIPTION_APPROVAL', 'WAITING_FOR_DEPLOYMENT_APPROVAL', 'DEPLOYMENT_APPROVED', 'EXPIRED')`,
		},
		{
			name: "pricing_policies type check",
			from: `((type)::text = ANY ((ARRAY['REGULAR'::character varying, 'DISCOUNT_PERCENTAGE'::character varying, 'AMOUNT_OFF'::character varying, 'PRICE_OVERRIDE'::character varying])::text[]))`,
			to:   `type in ('REGULAR', 'DISCOUNT_PERCENTAGE', 'AMOUNT_OFF', 'PRICE_OVERRIDE')`,
		},
		{
			name: "non-ANY expression unchanged",
			from: `(c1 > 1)`,
			to:   `(c1 > 1)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedFrom := NormalizeCheckExpr(tt.from)
			normalizedTo := NormalizeCheckExpr(tt.to)
			require.Equal(t, normalizedFrom, normalizedTo,
				"normalized expressions should match:\n  from: %s -> %s\n  to:   %s -> %s",
				tt.from, normalizedFrom, tt.to, normalizedTo)
		})
	}
}

// TestCheckDiffMode_ANYvsIN verifies that after inspect normalizes ANY(ARRAY[...])
// to IN (...), the diff correctly sees them as equivalent.
func TestCheckDiffMode_ANYvsIN(t *testing.T) {
	from := &schema.Table{
		Name: "payment_audit_records",
		Attrs: []schema.Attr{
			&schema.Check{
				Name: "payment_audit_records_status_check",
				Expr: `status IN ('PENDING', 'SUCCESS', 'FAILED')`,
			},
		},
	}
	to := &schema.Table{
		Name: "payment_audit_records",
		Attrs: []schema.Attr{
			&schema.Check{
				Name: "payment_audit_records_status_check",
				Expr: `status IN ('PENDING', 'SUCCESS', 'FAILED')`,
			},
		},
	}
	changes := CheckDiffMode(from, to, 0)
	require.Empty(t, changes, "identical normalized check constraints should produce no diff")
}

func TestExprLastIndex(t *testing.T) {
	tests := []struct {
		input   string
		wantIdx int
	}{
		{"", -1},
		{"()", 1},
		{"'('", 2},
		{"('(')", 4},
		{"('text')", 7},
		{"floor(x), y", 7},
		{"f(floor(x), y)", 13},
		{"f(floor(x), y, (z))", 18},
		{"f(x, (x*2)), y, (z)", 10},
		{"(a || ' ' || b)", 14},
		{"(a || ', ' || b)", 15},
		{"a || ', ' || b, x", 13},
		{"(a || ', ' || b), x", 15},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			idx := ExprLastIndex(tt.input)
			require.Equal(t, tt.wantIdx, idx)
		})
	}
}

func TestIsQuoted(t *testing.T) {
	tests := []struct {
		input  string
		quotes []byte
		want   bool
	}{
		{"''", []byte{'"', '\''}, true},
		{`""`, []byte{'"', '\''}, true},
		{"' '' \"\"'' '", []byte{'\''}, true},
		{"''''''''", []byte{'\''}, true},
		{"'foo'''", []byte{'\''}, true},
		{"'foo'''''", []byte{'\''}, true},
		{"'foo'', '''", []byte{'\''}, true},
		{"'foo bar'", []byte{'\''}, true},
		{`"never say \"never\""`, []byte{'"'}, true},
		{`"never say \"never\'"`, []byte{'"'}, true},
		{`'never say \"never\''`, []byte{'\''}, true},

		{"'", []byte{'"', '\''}, false},
		{`"`, []byte{'"', '\''}, false},
		{"'foo' ''", []byte{'\''}, false},
		{"'foo' ()  ''", []byte{'\''}, false},
		{"'foo', ''", []byte{'\''}, false},
		{"'foo', 'bar'", []byte{'\''}, false},
		{"'foo',\" 'bar'", []byte{'\''}, false},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			quoted := IsQuoted(tt.input, tt.quotes...)
			require.Equal(t, tt.want, quoted)
		})
	}
}

func TestReverseChanges(t *testing.T) {
	tests := []struct {
		input  []schema.Change
		expect []schema.Change
	}{
		{
			input: []schema.Change{
				(*schema.AddColumn)(nil),
			},
			expect: []schema.Change{
				(*schema.AddColumn)(nil),
			},
		},
		{
			input: []schema.Change{
				(*schema.AddColumn)(nil),
				(*schema.DropColumn)(nil),
			},
			expect: []schema.Change{
				(*schema.DropColumn)(nil),
				(*schema.AddColumn)(nil),
			},
		},
		{
			input: []schema.Change{
				(*schema.AddColumn)(nil),
				(*schema.ModifyColumn)(nil),
				(*schema.DropColumn)(nil),
			},
			expect: []schema.Change{
				(*schema.DropColumn)(nil),
				(*schema.ModifyColumn)(nil),
				(*schema.AddColumn)(nil),
			},
		},
		{
			input: []schema.Change{
				(*schema.AddColumn)(nil),
				(*schema.ModifyColumn)(nil),
				(*schema.DropColumn)(nil),
				(*schema.ModifyColumn)(nil),
			},
			expect: []schema.Change{
				(*schema.ModifyColumn)(nil),
				(*schema.DropColumn)(nil),
				(*schema.ModifyColumn)(nil),
				(*schema.AddColumn)(nil),
			},
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ReverseChanges(tt.input)
			require.Equal(t, tt.expect, tt.input)
		})
	}
}

func TestIsUint(t *testing.T) {
	require.True(t, IsUint("1"))
	require.False(t, IsUint("-1"))
	require.False(t, IsUint("1.2"))
	require.False(t, IsUint("1.2.3"))
}

func TestBodyDefChanged(t *testing.T) {
	for i, tt := range []struct {
		from, to string
		changed  bool
	}{
		{"", "", false},
		{"a", "a", false},
		{"a", "b", true},
		{"select 1;", "select 1", false},
		{"\nselect 1\n", "\nselect 1;", false},
		{"\nselect \n  \n1\n", "\nselect \n\n1;", false},
		{"\nselect \n  \n1\n'  '", "\nselect \n\n' ';", true},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			require.Equal(t, tt.changed, BodyDefChanged(tt.from, tt.to))
		})
	}
}
