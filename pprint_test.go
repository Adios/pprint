package pprint

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fmtTime time.Time

func (t fmtTime) String() string {
	return time.Time(t).Format("Jan _2 2006")
}

func TestMustToString(t *testing.T) {
	var (
		tm, _ = time.Parse("2006-01-02", "1989-12-27")

		tests = map[string]struct {
			in  interface{}
			out string
		}{
			"nil -> empty str": {nil, ""},
			"empty str":        {"", ""},
			"int":              {-10, "-10"},
			"string":           {"string", "string"},
			"struct -> %v":     {struct{}{}, "{}"},
			"ptr -> %v":        {(*string)(nil), "<nil>"},
			"time":             {tm, "1989-12-27 00:00:00 +0000 UTC"},
			"time + Stringer":  {fmtTime(tm), "Dec 27 1989"},
		}
	)
	for name, test := range tests {
		assert.Equal(t, test.out, MustToString(test.in), name)
	}
}

func TestColumn(t *testing.T) {
	type opts = []ColumnOpt

	tests := map[string]struct {
		colArgs  opts
		expected string
	}{
		"nil -> empty str": {opts{}, "%0s"},
		"fixed < 0":        {opts{WithWidth(-20)}, "%0s"},
		"fixed width":      {opts{WithWidth(20)}, "%20s"},
		"pad right":        {opts{WithWidth(20), WithLeftAlignment()}, "%-20s"},
	}
	for name, test := range tests {
		assert.Equal(t, test.expected, NewColumn(test.colArgs...).String(), name)
	}
}

func TestRowSchema(t *testing.T) {
	tests := map[string]struct {
		in       *Row
		expected *ColumnSchema
	}{
		"empty row has 0 schema": {NewRow(), NewSchemaFrom([]interface{}{})},
		"no data -> empty row":   {NewRow(WithRowData()), NewSchemaFrom([]interface{}{})},
		"nil input -> empty str": {
			NewRow(WithRowData(nil, -10, "")),
			NewRow(WithRowData("", 123, "")).Schema(),
		},
		"%1s case": {
			NewRow(WithRowData(0, 0, 0)),
			NewRow(WithRowData("1", "1", "1")).Schema(),
		},
		"enforce 2 columns": {
			NewRow(WithRowColumns(NewColumn(), NewColumn()), WithRowData(0, 0, 0)),
			NewRow(WithRowData("1", "1")).Schema(),
		},
		"enforce 3 columns": {
			NewRow(WithRowColumns(NewColumn(), NewColumn(), NewColumn()), WithRowData(0, 0)),
			NewRow(WithRowData("1", "1", "")).Schema(),
		},
	}
	for name, test := range tests {
		assert.Equal(t, test.expected, test.in.Schema(), name)
	}
}

func TestRowFmtArgs(t *testing.T) {
	type anys = []interface{}

	tm, _ := time.Parse("2006-01-02", "1989-12-27")
	tests := map[string]struct {
		in       *Row
		expected anys
	}{
		"empty row -> unit slice":                 {NewRow(), anys{}},
		"enforce 1 column & no data -> empty str": {NewRow(WithRowColumns(NewColumn())), anys{""}},
		"various data types": {
			NewRow(
				WithRowData(
					nil,
					struct{}{},
					-10,
					"",
					(*string)(nil),
					"no problem",
					tm,
					fmtTime(tm),
				),
			),
			anys{"", "{}", "-10", "", "<nil>", "no problem", "1989-12-27 00:00:00 +0000 UTC", "Dec 27 1989"},
		},
		"enforce 3 columns": {
			NewRow(
				WithRowColumns(NewColumn(), NewColumn(), NewColumn()),
				WithRowData(nil, -10),
			),
			anys{"", "-10", ""},
		},
		"enforce 2 columns": {
			NewRow(
				WithRowColumns(NewColumn(), NewColumn()),
				WithRowData(nil, -10, ""),
			),
			anys{"", "-10"},
		},
	}

	for name, test := range tests {
		assert.Equal(t, test.expected, test.in.FmtArgs(), name)
	}
}

func TestRowEachFmtStr(t *testing.T) {
	wontIterate := map[string]*Row{
		"empty row":                      NewRow(),
		"row with empty data":            NewRow(WithRowData()),
		"row with empty column":          NewRow(WithRowColumns()),
		"row with empty data & column":   NewRow(WithRowColumns(), WithRowData()),
		"row with empty column but data": NewRow(WithRowColumns(), WithRowData(nil, nil, nil)),
	}
	for name, test := range wontIterate {
		test.EachFmtStr(func(s string) { panic(name) })
	}

	tests := map[string]struct {
		in       *Row
		expected []string
	}{
		"default auto-width": {
			NewRow(WithRowData(nil, -10, "")),
			[]string{"%0s", "%3s", "%0s"},
		},
		"enforce 3 columns & no data": {
			NewRow(WithRowColumns(NewColumn(), NewColumn(), NewColumn())),
			[]string{"%0s", "%0s", "%0s"},
		},
		"enforce 3 columns & 3 data": {
			NewRow(WithRowColumns(NewColumn(), NewColumn(), NewColumn()), WithRowData(nil, -10, "")),
			[]string{"%0s", "%3s", "%0s"},
		},
		"enforce 2 columns": {
			NewRow(WithRowColumns(NewColumn(), NewColumn(), NewColumn()), WithRowData(nil, -10)),
			[]string{"%0s", "%3s", "%0s"},
		},
		"enforce 3 columns": {
			NewRow(WithRowColumns(NewColumn(), NewColumn()), WithRowData(nil, -10, "")),
			[]string{"%0s", "%3s"},
		},
	}

	for name, test := range tests {
		i := 0
		test.in.EachFmtStr(func(s string) {
			assert.Equal(t, test.expected[i], s, name)
			i = i + 1
		})
	}
}

func TestRowEachFmtStrWithSchemaInheritance(t *testing.T) {
	assert := assert.New(t)

	a := NewRow(
		WithRowColumns(
			NewColumn(WithWidth(5)),
			NewColumn(WithWidth(5), WithLeftAlignment()),
			NewColumn(WithLeftAlignment()),
			NewColumn(),
		),
		WithRowData("123456", "123456", "123456", "123456"),
	)

	current, i := []string{"%5s", "%-5s", "%-6s", "%6s"}, 0
	a.EachFmtStr(func(s string) {
		assert.Equal(current[i], s)
		i = i + 1
	})

	// B inherits A
	b := NewRow(
		WithRowSchema(a.Schema()),
		WithRowData("1234567890", "1234567890", "1234567890", "1234567890"),
	)
	assert.Same(b.Schema(), a.Schema())

	afterInherited, i := []string{"%5s", "%-5s", "%-10s", "%10s"}, 0
	a.EachFmtStr(func(s string) {
		assert.Equal(afterInherited[i], s, "B should update A's FmtStr")
		i = i + 1
	})
}

func TestNodeInternalTreeCreation(t *testing.T) {
	var (
		assert = assert.New(t)
		n      = NewNode()
		n0, _  = n.Push()
		n1, _  = n.Push()
		n00, _ = n0.Push()
		n01, _ = n0.Push()
		n10, _ = n1.Push()
		n11, _ = n1.Push()
	)

	assert.Equal(2, n.NodesCount())
	assert.Same(n0, n.nodes[0])
	assert.Same(n1, n.nodes[1])
	assert.Same(n0.Parent(), n)
	assert.Same(n1.Parent(), n)

	assert.Equal(2, n0.NodesCount())
	assert.Same(n00, n0.nodes[0])
	assert.Same(n01, n0.nodes[1])
	assert.Same(n00.Parent(), n0)
	assert.Same(n01.Parent(), n0)

	assert.Equal(2, n1.NodesCount())
	assert.Same(n10, n1.nodes[0])
	assert.Same(n11, n1.nodes[1])
	assert.Same(n10.Parent(), n1)
	assert.Same(n11.Parent(), n1)
}

func TestNodeWalk(t *testing.T) {
	assert := assert.New(t)

	root := NewNode()
	a, _ := root.Push()
	b, _ := root.Push()
	c, _ := root.Push()
	o, _ := a.Push()
	p, _ := a.Push()

	order, i := []*Node{a, o, p, b, c}, 0
	root.Walk(func(c *Node) {
		assert.Same(order[i], c)
		i += 1
	})

	// Merge another tree
	anotherRoot := NewNode(WithSchema(root.Schema()))
	x, _ := anotherRoot.Push()
	y, _ := anotherRoot.Push()

	root.PushNode(anotherRoot)

	merged, i := []*Node{a, o, p, b, c, anotherRoot, x, y}, 0
	root.Walk(func(c *Node) {
		assert.Same(merged[i], c)
		i += 1
	})

	// Traverse subtree
	sub, i := []*Node{x, y}, 0
	anotherRoot.Walk(func(c *Node) {
		assert.Same(sub[i], c)
		i += 1
	})
}

func TestNodePushNode(t *testing.T) {
	assert := assert.New(t)

	{
		a := NewNode()
		_, err := a.PushNode(nil)
		assert.EqualError(err, "PushNode: nil incoming")
	}
	{
		// A has no schema, B has no row nor node schema
		a := NewNode()
		_, err := a.PushNode(NewNode(WithRow(nil)))
		assert.EqualError(err, "PushNode: no schema to set")
	}
	{
		// A has no schema, B has no row nor node schema
		a := NewNode()
		b := NewNode()
		_, err := a.PushNode(b)
		assert.EqualError(err, "PushNode: no schema to set")
	}
	{
		// A has no schema, B has no row, but has node schema
		a := NewNode()
		b := NewNode()
		b.Push()
		_, err := a.PushNode(b)
		assert.NoError(err)
	}
	{
		// A has no schema, B has no row, but has node schema
		a := NewNode()
		b := NewNode(WithColumns(NewColumn()))
		assert.Nil(b.Row())
		_, err := a.PushNode(b)
		assert.NoError(err)
		assert.NotNil(b.Row(), "A creates an empty Row for B")
		assert.Same(b.Schema(), a.Schema(), "A's node schema = B's node schema")
		assert.Same(b.Row().Schema(), a.Schema(), "A's node schema = B's row schema")
	}
	{
		// A has no schema, B has row, but no node schema
		a := NewNode()
		b := NewNode(WithRow(NewRow()))
		_, err := a.PushNode(b)
		assert.NoError(err)
	}
	{
		// A has no schema, B has row and node schema
		a := NewNode()
		b := NewNode(WithColumns(NewColumn()), WithRow(NewRow()))
		assert.NotNil(b.Schema())
		assert.NotNil(b.Row().Schema())
		_, err := a.PushNode(b)
		assert.NoError(err)
		// A looks for B's row schema over node schema.
		assert.Same(b.Row().Schema(), a.Schema())
		assert.NotSame(b.Schema(), a.Schema())
	}
	{
		// A has schema, B has no row nor schema.
		a := NewNode()
		a.Push(1)
		b := NewNode()
		_, err := a.PushNode(b)
		assert.NoError(err)
	}
	{
		// A has schema, B has no row, but has node schema.
		a := NewNode()
		a.Push(1)
		b := NewNode(WithColumns(NewColumn()))
		assert.Nil(b.Row())
		_, err := a.PushNode(b)
		assert.NoError(err)
		assert.NotNil(b.Row(), "A creates an empty Row for B")
		assert.NotSame(b.Schema(), a.Schema(), "B's node schema doesn't changed")
		assert.Same(b.Row().Schema(), a.Schema(), "A's node schema = B's row schema")
	}
	{
		// A has schema, B has row but different schema.
		a := NewNode()
		a.Push(1)
		b := NewNode(WithRow(NewRow()))
		_, err := a.PushNode(b)
		assert.EqualError(err, "PushNode: row of the incoming node doesn't match my node schema")
	}
	{
		// A has schema, B has row with same schema.
		a := NewNode()
		a.Push(1)
		b := NewNode(WithRow(NewRow(WithRowSchema(a.Schema()))))
		c, err := a.PushNode(b)
		assert.NoError(err)
		assert.Same(b.Row().Schema(), a.Schema())

		// Create another root
		p := NewNode()
		q := NewNode(WithRow(NewRow(WithRowSchema(a.Schema()))))
		_, err = p.PushNode(q)
		assert.NoError(err)

		// Merge and check all nodes
		_, err = c.PushNode(p)
		assert.NoError(err)

		assert.Same(p.Row().Schema(), a.Schema())
		assert.Same(b.Row().Schema(), q.Row().Schema())
	}
}

func TestNodePush(t *testing.T) {
	var assert = assert.New(t)

	{
		// Check trivial push
		a := NewNode()
		b, err := a.Push()
		assert.NoError(err)
		assert.Same(b.Parent(), a)
		assert.Same(b.Row().Schema(), a.Schema())
		assert.Equal(1, a.NodesCount())
	}
	{
		// Check inheritance
		a := NewNode()
		assert.Nil(a.Schema())
		assert.Nil(a.Row())

		b, _ := a.Push("push")
		assert.Same(b.Parent(), a)
		assert.Same(b.Row().Schema(), a.Schema(), "A's node schema = B's row schema")
		assert.Nil(b.Schema(), "B has no node schema")

		c, _ := a.Push("push")
		assert.Same(c.Parent(), a)
		assert.Same(c.Row().Schema(), b.Row().Schema(), "C's row schema = B's row schema")
		assert.Nil(c.Schema())

		d, _ := b.Push("push")
		assert.Same(d.Row().Schema(), b.Schema(), "D's row schema = B's node schema")
	}

	var checkSchemaStr = func(n *Node, fn func(i int, s string)) {
		for i, col := range n.Schema().cols {
			fn(i, col.String())
		}
	}

	{
		// Check fmtstr gets enforced by node schema
		a := NewNode(WithColumns(NewColumn(WithWidth(20))))
		a.Push("1", "12", "123")
		expected := []string{"%20s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(expected[i], s)
		})
	}
	{
		// Check fmtstr gets enforced and updates
		a := NewNode()
		a.Push("1", "12", "123")
		expected := []string{"%1s", "%2s", "%3s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(expected[i], s, "auto create schema of 3 columns")
		})

		b, _ := a.Push()
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(expected[i], s, "push empty alters nothing")
		})

		a.Push("", "123", "1234", "1234")
		expected = []string{"%1s", "%3s", "%4s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(expected[i], s, "be enforced to 3 columns & longer width updated")
		})

		// node -> b -> a, updates to b also updates to a
		b.Push("12345", "12345", "12345", "12345")
		expected = []string{"%5s", "%5s", "%5s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(expected[i], s, "A should be updated")
		})
	}
}

func TestNodeSortFailed(t *testing.T) {
	assert := assert.New(t)

	{
		// Empty node have no schema, anything results an error
		n := NewNode()
		err := n.Sort(0)
		assert.EqualError(err, "Sort: column 0 doesn't exist")
	}
	{
		// Over index range
		n := NewNode()
		n.Push()
		err := n.Sort(1)
		assert.EqualError(err, "Sort: column 1 doesn't exist")
	}
	{
		n := NewNode()
		n.Push(0, 1)
		n.Push(0, "")
		err := n.Sort(1)
		assert.EqualError(err, "createSortableOn: column 1 doesn't contain identical value type")
	}
	{
		n := NewNode()
		n.Push(0, 1)
		n.Push(0, 2)
		assert.Panics(func() {
			n.Sort(0, WithCmpMatchers(func(a interface{}) CmpFn {
				// force invalid type comparison
				return func(a, b interface{}) bool {
					return a.(string) < b.(string)
				}
			}))
		})
	}
}

func TestNodeSortSuccessOneOrNoItem(t *testing.T) {
	assert := assert.New(t)

	{
		n := NewNode(WithColumns(NewColumn()))
		err := n.Sort(0)
		assert.NoError(err, "sort 0 item with column schema won't trigger error")
	}
	{
		n := NewNode()
		n.Push(-9, "violation")
		err := n.Sort(0)
		assert.NoError(err)
	}
	{
		n := NewNode()
		n.Push(0, 2)
		err := n.Sort(0, WithCmpMatchers(func(a interface{}) CmpFn {
			// force invalid type comparison
			return func(a, b interface{}) bool {
				return a.(string) < b.(string)
			}
		}))
		assert.NoError(err, "sort 1 item won't trigger cmp matching")
	}
}

func TestNodeSortSuccess(t *testing.T) {
	type (
		anys = []interface{}
		key  int
	)

	var (
		pt = func(date string) time.Time {
			t, _ := time.Parse("2006-01-02", date)
			return t
		}
		data = map[key]anys{
			0: {-9, "violation", pt("1989-12-27")},
			1: {0, "progress", pt("1988-08-17")},
			2: {1227, "alcohol", pt("1993-02-13")},
			3: {712, "animal", pt("1999-07-01")},
			4: {712, "flawed", pt("1993-02-13")},
		}
	)

	tests := map[string]struct {
		run      func(*Node)
		expected []key
	}{
		"sort int": {
			func(n *Node) { n.Sort(0) },
			[]key{0, 1, 3, 4, 2},
		},
		"sort int reversely": {
			func(n *Node) { n.Sort(0, WithDescending()) },
			[]key{2, 4, 3, 1, 0},
		},
		"sort string": {
			func(n *Node) { n.Sort(1) },
			[]key{2, 3, 4, 1, 0},
		},
		"sort time": {
			func(n *Node) { n.Sort(2) },
			[]key{1, 0, 2, 4, 3},
		},
		"sort with custom cmp": {
			func(n *Node) {
				n.Sort(0, WithCmpMatchers(func(a interface{}) CmpFn {
					return func(a, b interface{}) bool { return a.(int) > b.(int) }
				}))
			},
			[]key{2, 3, 4, 1, 0},
		},
		"combine multiple sorts": {
			func(n *Node) {
				n.Sort(0)
				n.Sort(1, WithDescending())
				n.Sort(0)
				n.Sort(2)
			},
			[]key{1, 0, 4, 2, 3},
		},
	}

	for name, test := range tests {
		n := NewNode()
		for i := 0; i < 5; i++ {
			n.Push(data[key(i)]...)
		}
		test.run(n)
		i := 0
		n.EachNode(func(c *Node) {
			dataKey := test.expected[i]
			assert.Equal(t, data[dataKey], c.Row().fields, name)
			i += 1
		})
	}
}

func TestPrintingRunRow(t *testing.T) {
	type (
		pOpts = []PrintingOpt
		rOpts = []RowOpt
	)

	tests := map[string]struct {
		pOpts pOpts
		rOpts rOpts
		out   string
	}{
		"empty row -> ": {
			pOpts{}, rOpts{}, "",
		},
		"1 column empty string -> \n": {
			pOpts{}, rOpts{WithRowColumns(NewColumn())}, "\n",
		},
		"various types": {
			pOpts{},
			rOpts{WithRowData(1, "hello", nil, 123.1, (*string)(nil))},
			"1 hello  123.1 <nil>\n",
		},
		"various types without separator": {
			pOpts{WithColSep("")},
			rOpts{WithRowData(1, "hello", nil, 123.1, (*string)(nil))},
			"1hello123.1<nil>\n",
		},
		"various types with my separator and no line break": {
			pOpts{WithColSep("##"), WithLineBrk("")},
			rOpts{WithRowData(1, "hello", nil, 123.1, (*string)(nil))},
			"1##hello####123.1##<nil>",
		},
	}

	for name, test := range tests {
		var s strings.Builder
		test.pOpts = append(test.pOpts, WithWriter(&s))

		NewPrinting(test.pOpts...).RunRow(NewRow(test.rOpts...))
		assert.Equal(t, test.out, s.String(), name)
	}
}

func TestPrintingRunNode(t *testing.T) {
	var (
		assert = assert.New(t)

		s strings.Builder
		p = NewPrinting(WithWriter(&s))
	)

	{
		p.RunNode(NewNode())
		assert.Equal("", s.String(), "empty node prints nothing")
	}
	{
		a := NewNode()
		a.Push()
		p.RunNode(a)
		assert.Equal("", s.String(), "node with empty row prints nothing")
	}
	{
		a := NewNode()
		a.Push(nil)
		p.RunNode(a)
		assert.Equal("\n", s.String(), "1 column node with empty string")
	}
	{
		// Complex case with field shrink and descendants
		s.Reset()
		a := NewNode()
		b, _ := a.Push("1", "12", "123")
		a.Push()
		a.Push("", "123", "1234", "1234")
		b.Push("12345", "12345", "12345", "12345")
		p.RunNode(a)
		assert.Equal(
			"    1    12   123\n"+
				"12345 12345 12345\n"+
				"                 \n"+
				"        123  1234\n",
			s.String(),
		)
		s.Reset()

		// Print subtree
		p.RunNode(b)
		assert.Equal(
			"    1    12   123\n"+
				"12345 12345 12345\n",
			s.String(),
		)
	}
}

func TestRowString(t *testing.T) {
	tm, _ := time.Parse("2006-01-02", "1989-12-27")
	tests := map[string]struct {
		in       *Row
		expected string
	}{
		"empty row -> ":    {NewRow(), ""},
		"1 column row -> ": {NewRow(WithRowColumns(NewColumn())), ""},
		"auto-width various types": {
			NewRow(
				WithRowData(
					nil,
					struct{}{},
					-10,
					"",
					(*string)(nil),
					"no problem",
					tm,
					fmtTime(tm),
				),
			),
			" {} -10  <nil> no problem 1989-12-27 00:00:00 +0000 UTC Dec 27 1989",
		},
		"typesetting and enforce 5 columns": {
			NewRow(
				WithRowColumns(
					NewColumn(WithWidth(3)),
					NewColumn(WithWidth(0)),
					NewColumn(WithWidth(0), WithLeftAlignment()),
					NewColumn(WithWidth(5), WithLeftAlignment()),
					NewColumn(WithLeftAlignment()),
				),
				WithRowData(
					nil,
					"nowidth",
					"nowidthleft",
					-10,
					"no problem",
					"this field will be discarded",
				),
			),
			"    nowidth nowidthleft -10   no problem",
		},
		"enforce 2 columns": {
			NewRow(WithRowColumns(NewColumn(), NewColumn()), WithRowData(0)),
			"0 ",
		},
	}

	for name, test := range tests {
		assert.Equal(t, test.expected, test.in.String(), name)
	}
}

func TestNodeString(t *testing.T) {
	var (
		assert = assert.New(t)

		pt = func(date string) time.Time {
			t, _ := time.Parse("2006-01-02", date)
			return t
		}
		data = [][]interface{}{
			{-9, "violation", pt("1989-12-27"), "this"},
			{0, "progress", pt("1988-08-17"), "column"},
			{1227, "alcohol", pt("1993-02-13"), "hello"},
			{712, "animal", pt("1999-07-01"), "returns"},
			{712, "flawed", pt("1993-02-13"), "error"},
			{12345, "ok", nil, "wont"},
		}
	)

	{
		a := NewNode()
		assert.Equal("", a.String(), "empty node prints nothing")
	}
	{
		// complex case
		a := NewNode(WithColumns(
			NewColumn(WithLeftAlignment()),
			NewColumn(WithWidth(16)),
			NewColumn(),
			NewColumn(WithWidth(0)),
		))
		a.Push(data[0]...)
		a.Push(data[1]...)
		b, _ := a.Push(data[2]...)
		a.Push(data[3]...)
		a.Push(data[4]...)
		b.Push(data[5]...)
		assert.Equal(
			`-9           violation 1989-12-27 00:00:00 +0000 UTC this
0             progress 1988-08-17 00:00:00 +0000 UTC column
1227           alcohol 1993-02-13 00:00:00 +0000 UTC hello
12345               ok                               wont
712             animal 1999-07-01 00:00:00 +0000 UTC returns
712             flawed 1993-02-13 00:00:00 +0000 UTC error
`,
			a.String(),
		)
	}
}
