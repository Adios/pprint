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
			"nil to be empty":       {nil, ""},
			"empty to empty":        {"", ""},
			"int to string":         {-10, "-10"},
			"normal string":         {"string", "string"},
			"struct to be %#v":      {struct{}{}, "{}"},
			"ptr to be %#v":         {(*string)(nil), "<nil>"},
			"time without Stringer": {tm, "1989-12-27 00:00:00 +0000 UTC"},
			"time with Stringer":    {fmtTime(tm), "Dec 27 1989"},
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
		"nil column":       {opts{}, "%0s"},
		"negative width":   {opts{WithWidth(-20)}, "%0s"},
		"normal width":     {opts{WithWidth(20)}, "%20s"},
		"padding to right": {opts{WithWidth(20), WithLeftAlignment()}, "%-20s"},
	}

	for name, test := range tests {
		assert.Equal(t, test.expected, NewColumn(test.colArgs...).String(), name)
	}
}

func TestRowSchema(t *testing.T) {
	tests := map[string]struct {
		in       *Row
		expected interface{}
	}{
		"nil row has 0 input schema":  {NewRow(), NewSchemaFrom([]interface{}{})},
		"nil data has 0 input schema": {NewRow(WithData()), NewSchemaFrom([]interface{}{})},
		"nil fields to be empty str": {
			NewRow(WithData(nil, -10, "")),
			NewRow(WithData("", 123, "")),
		},
		"normal %1s schema": {
			NewRow(WithData(0, 0, 0)),
			NewRow(WithData("1", "1", "1")),
		},
		"3 fields to be cut to 2 columns": {
			NewRow(WithColumns(NewColumn(), NewColumn()), WithData(0, 0, 0)),
			NewRow(WithData("1", "1")),
		},
		"2 fields to be expanded to 3 columns": {
			NewRow(WithColumns(NewColumn(), NewColumn(), NewColumn()), WithData(0, 0)),
			NewRow(WithData("1", "1", "")),
		},
	}
	for name, test := range tests {
		var scm *ColumnSchema

		switch v := test.expected.(type) {
		case *ColumnSchema:
			scm = v
		case *Row:
			scm = v.Schema()
		}

		assert.Equal(t, scm, test.in.Schema(), name)
	}
}

func TestRowFmtArgs(t *testing.T) {
	type anys = []interface{}

	tm, _ := time.Parse("2006-01-02", "1989-12-27")
	tests := map[string]struct {
		in       *Row
		expected anys
	}{
		"nil data row returns empty slice":                {NewRow(), anys{}},
		"no data but have one column means lacks a field": {NewRow(WithColumns(NewColumn())), anys{""}},
		"general case": {
			NewRow(
				WithData(
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
		"lacks a field": {
			NewRow(
				WithColumns(NewColumn(), NewColumn(), NewColumn()),
				WithData(nil, -10),
			),
			anys{"", "-10", ""},
		},
		"overs a field": {
			NewRow(
				WithColumns(NewColumn(), NewColumn()),
				WithData(nil, -10, ""),
			),
			anys{"", "-10"},
		},
	}

	for name, test := range tests {
		assert.Equal(t, test.expected, test.in.FmtArgs(), name)
	}
}

func TestRowEachFmtStr(t *testing.T) {
	wontIterates := map[string]*Row{
		"nil data row":                            NewRow(),
		"nil input data row":                      NewRow(WithData()),
		"nil input for row column":                NewRow(WithColumns()),
		"nil input for both row columns and data": NewRow(WithColumns(), WithData()),
		"nil data input":                          NewRow(WithColumns(), WithData(nil, nil, nil)),
	}
	for name, test := range wontIterates {
		test.EachFmtStr(func(s string) { panic(name) })
	}

	tests := map[string]struct {
		in       *Row
		expected []string
	}{
		"auto schema": {
			NewRow(WithData(nil, -10, "")),
			[]string{"%0s", "%3s", "%0s"},
		},
		"initial predefined schema": {
			NewRow(WithColumns(NewColumn(), NewColumn(), NewColumn())),
			[]string{"%0s", "%0s", "%0s"},
		},
		"normal case": {
			NewRow(WithColumns(NewColumn(), NewColumn(), NewColumn()), WithData(nil, -10, "")),
			[]string{"%0s", "%3s", "%0s"},
		},
		"lacks a field": {
			NewRow(WithColumns(NewColumn(), NewColumn(), NewColumn()), WithData(nil, -10)),
			[]string{"%0s", "%3s", "%0s"},
		},
		"overs a field": {
			NewRow(WithColumns(NewColumn(), NewColumn()), WithData(nil, -10, "")),
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
		WithColumns(
			NewColumn(WithWidth(5)),
			NewColumn(WithWidth(5), WithLeftAlignment()),
			NewColumn(WithLeftAlignment()),
			NewColumn(),
		),
		WithData("123456", "123456", "123456", "123456"),
	)

	before, i := []string{"%5s", "%-5s", "%-6s", "%6s"}, 0
	a.EachFmtStr(func(s string) {
		assert.Equal(before[i], s, "real width and fixed width")
		i = i + 1
	})

	b := NewRow(
		WithSchema(a.Schema()),
		WithData("1234567890", "1234567890", "1234567890", "1234567890"),
	)
	assert.Same(b.Schema(), a.Schema(), "same instance")

	after, i := []string{"%5s", "%-5s", "%-10s", "%10s"}, 0
	a.EachFmtStr(func(s string) {
		assert.Equal(after[i], s, "update b will update a's schema for auto width columns")
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

	anotherRoot := NewNode(WithRow(NewRow(WithSchema(root.Schema()))))
	x, _ := anotherRoot.Push()
	y, _ := anotherRoot.Push()

	root.PushNode(anotherRoot)

	order, i := []*Node{a, o, p, b, c, anotherRoot, x, y}, 0
	root.Walk(func(c *Node) {
		assert.Same(order[i], c, "follow the order with a merged subtree")
		i += 1
	})

	subOrder, i := []*Node{x, y}, 0
	anotherRoot.Walk(func(c *Node) {
		assert.Same(subOrder[i], c, "only traverse merged subtree")
		i += 1
	})
}

func TestNodePushNodeSuccess(t *testing.T) {
	assert := assert.New(t)

	{
		a := NewNode()
		b := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		c, err := a.PushNode(b)
		assert.NoError(err, "Empty node accepts to be pushed with a non-empty node")
		assert.Same(c, b, "Returned node is the same as that being pushed")
		assert.Same(c.Parent(), a, "A will be B(C)'s parent")
		assert.Same(c, a.nodes[0], "B(C) will be A's child")
	}
	{
		a := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		// by current design, this won't happen
		wont := NewRow()
		wont.schema = nil
		b := NewNode(WithRow(wont))
		c, err := a.PushNode(b)
		assert.NoError(err, "Empty node is allowed to be pushed to a non-empty node")
		assert.Same(c.Schema(), a.Schema(), "B(C) will inherit A")
	}
	{
		a := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		b := NewNode(WithRow(NewRow(WithSchema(a.Schema()))))
		c, err := a.PushNode(b)
		assert.NoError(err, "Accepts nodes with identical column schema")
		assert.Same(c, b)
		assert.Same(c.parent, a)
		assert.Same(c, a.nodes[0])
	}
}

func TestNodePushNodeFailed(t *testing.T) {
	assert := assert.New(t)

	{
		a := NewNode()
		_, err := a.PushNode(nil)
		assert.EqualError(err, "PushNode: incoming can't be nil")
	}
	{
		a := NewNode()
		_, err := a.PushNode(NewNode(WithRow(nil)))
		assert.EqualError(err, "PushNode: can't add empty node", "node with nil row is considered as empty")
	}
	{
		a := NewNode()
		b := NewNode()
		_, err := a.PushNode(b)
		assert.EqualError(err, "PushNode: can't add empty node", "since both nodes are empty")
	}
	{
		a := NewNode()
		// by current design, this won't happen
		wont := NewRow()
		wont.schema = nil
		b := NewNode(WithRow(wont))
		_, err := a.PushNode(b)
		assert.EqualError(err, "PushNode: both nodes have no schemas")
	}
	{
		a := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		b := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		_, err := a.PushNode(b)
		assert.EqualError(err, "PushNode: incoming node must have the same schema")
	}
}

func TestNodePush(t *testing.T) {
	var (
		assert         = assert.New(t)
		checkSchemaStr = func(n *Node, fn func(i int, s string)) {
			for i, col := range n.Schema().cols {
				fn(i, col.String())
			}
		}
	)

	{
		a := NewNode()
		b, err := a.Push()
		assert.NoError(err, "results a node with 0 column schema, a trivial node")
		assert.Same(b.Parent(), a, "A will be B's parrent")
		assert.Same(b.Schema(), a.Schema(), "A and B shares the same schema")
		assert.Equal(1, a.NodesCount())
	}
	{
		a := NewNode(WithRow(NewRow(WithColumns(NewColumn(WithWidth(20))))))
		a.Push("1", "12", "123")
		initial := []string{"%20s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(initial[i], s, "push data into a predefined node")
		})
	}
	{
		a := NewNode(WithRow(NewRow(WithData(0, 1))))
		initial := []string{"%1s", "%1s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(initial[i], s, "initial with a node that alreadys has a row")
		})
		assert.Equal(0, a.NodesCount(), "make sure it has no children")
		a.Push("1", "12", "123")
		after := []string{"%1s", "%2s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(after[i], s, "even a pre-rowed node, the push works as expected")
		})
	}
	{
		a := NewNode()
		a.Push("1", "12", "123")
		initial := []string{"%1s", "%2s", "%3s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(initial[i], s, "initial push automatically creates a schema of 3 columns")
		})
		b, _ := a.Push()
		state1 := initial
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(state1[i], s, "pushed empty node alters no schema since its widths are 0s")
		})
		a.Push("", "123", "1234", "1234")
		state2 := []string{"%1s", "%3s", "%4s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(state2[i], s, "data with longer width updates schema, over-field is discarded")
		})
		b.Push("12345", "12345", "12345", "12345")
		state3 := []string{"%5s", "%5s", "%5s"}
		checkSchemaStr(a, func(i int, s string) {
			assert.Equal(state3[i], s, "put new child under B also updates schema to A")
		})
	}
}

func TestNodeSortFailed(t *testing.T) {
	assert := assert.New(t)

	{
		n := NewNode()
		err := n.Sort(0)
		assert.EqualError(err, "Sort: no such field", "empty node has no fields")
	}
	{
		n := NewNode()
		n.Push()
		err := n.Sort(1)
		assert.EqualError(err, "Sort: no such field")
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
		n := NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
		err := n.Sort(0)
		assert.NoError(err, "have column, no items, it returns immediately")
	}
	{
		n := NewNode()
		n.Push(-9, "violation")
		err := n.Sort(0)
		assert.NoError(err, "one item, it returns immediately")
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
		assert.NoError(err, "one item, it returns immediately, no trigger cmp matcher that panics")
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
		"empty row prints nothing": {
			pOpts{}, rOpts{}, "",
		},
		"equals empty string, print only new line": {
			pOpts{}, rOpts{WithColumns(NewColumn())}, "\n",
		},
		"default case": {
			pOpts{},
			rOpts{WithData(1, "hello", nil, 123.1, (*string)(nil))},
			"1 hello  123.1 <nil>\n",
		},
		"without column separator": {
			pOpts{WithColSep("")},
			rOpts{WithData(1, "hello", nil, 123.1, (*string)(nil))},
			"1hello123.1<nil>\n",
		},
		"custom column separator and line break": {
			pOpts{WithColSep("##"), WithLineBrk("")},
			rOpts{WithData(1, "hello", nil, 123.1, (*string)(nil))},
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
		assert.Equal("", s.String(), "empty row prints nothing")
	}
	{
		a := NewNode()
		a.Push(nil)
		p.RunNode(a)
		assert.Equal("\n", s.String(), "one column node with empty string")
	}
	{
		s.Reset()
		a := NewNode()
		b, _ := a.Push("1", "12", "123")
		a.Push()
		a.Push("", "123", "1234", "1234")
		b.Push("12345", "12345", "12345", "12345")
		p.RunNode(a)
		assert.Equal(
			"    1    12   123\n"+"12345 12345 12345\n"+
				"                 \n"+"        123  1234\n",
			s.String(),
			"complex case with field cutting and descendant",
		)
		s.Reset()
		p.RunNode(b)
		assert.Equal(
			"    1    12   123\n"+"12345 12345 12345\n",
			s.String(),
			"a non-root node should also output itself",
		)
	}
}

func TestRowString(t *testing.T) {
	tm, _ := time.Parse("2006-01-02", "1989-12-27")
	tests := map[string]struct {
		in       *Row
		expected string
	}{
		"nil data outputs empty string": {NewRow(), ""},
		"no data outpus empty string":   {NewRow(WithColumns(NewColumn())), ""},
		"all auto width fields": {
			NewRow(
				WithData(
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
		"custom specified fields and overs a field": {
			NewRow(
				WithColumns(
					NewColumn(WithWidth(3)),
					NewColumn(WithWidth(0)),
					NewColumn(WithWidth(0), WithLeftAlignment()),
					NewColumn(WithWidth(5), WithLeftAlignment()),
					NewColumn(WithLeftAlignment()),
				),
				WithData(
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
		"lacks a field": {
			NewRow(WithColumns(NewColumn(), NewColumn()), WithData(0)),
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
			{1227, "alcohol", pt("1993-02-13")},
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
		a := NewNode(WithRow(NewRow(WithColumns(
			NewColumn(WithLeftAlignment()),
			NewColumn(WithWidth(16)),
			NewColumn(),
			NewColumn(WithWidth(0)),
		))))
		a.Push(data[0]...)
		a.Push(data[1]...)
		b, _ := a.Push(data[2]...)
		a.Push(data[3]...)
		a.Push(data[4]...)
		b.Push(data[5]...)
		assert.Equal(
			`-9           violation 1989-12-27 00:00:00 +0000 UTC this
0             progress 1988-08-17 00:00:00 +0000 UTC column
1227           alcohol 1993-02-13 00:00:00 +0000 UTC 
12345               ok                               wont
712             animal 1999-07-01 00:00:00 +0000 UTC returns
712             flawed 1993-02-13 00:00:00 +0000 UTC error
`,
			a.String(),
			"complext case with lacking field and descendants",
		)
	}
}
