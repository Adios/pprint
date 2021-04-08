package pprint

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNodeString(t *testing.T) {
	n := NewNode()
	assert.Equal(t, "", n.String())

	var (
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
		}
	)

	n = NewNode(WithRow(NewRow(WithColumns(
		NewColumn(WithLeftAlignment()),
		NewColumn(WithWidth(16)),
		NewColumn(),
		NewColumn(WithWidth(0)),
	))))

	n.Push(data[0]...)
	n.Push(data[1]...)
	n02, _ := n.Push(data[2]...)
	n.Push(data[3]...)
	n.Push(data[4]...)
	n02.Push(12345, "ok", nil, "wont")

	assert.Equal(t, `-9           violation 1989-12-27 00:00:00 +0000 UTC this
0             progress 1988-08-17 00:00:00 +0000 UTC column
1227           alcohol 1993-02-13 00:00:00 +0000 UTC 
12345               ok                               wont
712             animal 1999-07-01 00:00:00 +0000 UTC returns
712             flawed 1993-02-13 00:00:00 +0000 UTC error
`, n.String())
}

func TestPrintingRunNode(t *testing.T) {
	var s strings.Builder

	p := NewPrinting(WithWriter(&s))
	p.RunNode(NewNode())
	assert.Equal(t, 0, s.Len())

	n := NewNode()
	n.Push()
	p.RunNode(n)
	assert.Equal(t, 0, s.Len())

	n = NewNode()
	n.Push(nil)
	p.RunNode(n)
	assert.Equal(t, "\n", s.String())

	s.Reset()
	n = NewNode()
	n0, _ := n.Push("1", "12", "123")
	n.Push()
	n.Push("", "123", "1234", "1234")
	n0.Push("12345", "12345", "12345", "12345")

	p.RunNode(n)
	assert.Equal(t, `    1    12   123
12345 12345 12345
                 
        123  1234
`, s.String())

	s.Reset()
	p.RunNode(n0)
	assert.Equal(t, `    1    12   123
12345 12345 12345
`, s.String())
}

func TestPrintingRunRow(t *testing.T) {
	var s strings.Builder

	p := NewPrinting(WithWriter(&s))
	p.RunRow(NewRow())
	assert.Equal(t, 0, s.Len())

	p.RunRow(NewRow(WithData(1)))
	assert.Equal(t, "1\n", s.String())

	s.Reset()
	p.RunRow(NewRow(WithData(1, "hello", nil, 123.1, (*string)(nil))))
	assert.Equal(t, "1 hello  123.1 <nil>\n", s.String())

	s.Reset()
	p = NewPrinting(WithWriter(&s), WithSeparator(""))
	p.RunRow(NewRow(WithData(1, "hello", nil, 123.1, (*string)(nil))))
	assert.Equal(t, "1hello123.1<nil>\n", s.String())

	s.Reset()
	p = NewPrinting(WithWriter(&s), WithSeparator("##"), WithoutLf())
	p.RunRow(NewRow(WithData(1, "hello", nil, 123.1, (*string)(nil))))
	assert.Equal(t, "1##hello####123.1##<nil>", s.String())
}

func TestRowString(t *testing.T) {
	r := NewRow()
	assert.Equal(t, "", r.String())

	tm := time.Date(1989, 12, 27, 0, 0, 0, 0, time.UTC)

	r = NewRow(
		WithData(
			nil,
			testStruct{},
			-10,
			"",
			(*string)(nil),
			"no problem",
			tm,
			testTime(tm),
		),
	)
	assert.Equal(t, " {} -10  <nil> no problem 1989-12-27 00:00:00 +0000 UTC Dec 27 1989", r.String())

	r = NewRow(
		WithColumns(
			NewColumn(WithWidth(3)),
			NewColumn(WithWidth(0)),
			NewColumn(WithWidth(0), WithLeftAlignment()),
			NewColumn(WithWidth(5), WithLeftAlignment()),
			NewColumn(WithLeftAlignment()),
		),
		WithData(
			nil,
			"width0",
			"width0left",
			-10,
			"no problem",
		),
	)
	assert.Equal(t, "    width0 width0left -10   no problem", r.String())
}

func TestCreateNodeItemAdapter(t *testing.T) {
	adpt, err := CreateNodeItemAdapter(nil, 0)
	assert.Equal(t, NodeItemAdapter{}, adpt)
	assert.EqualError(t, err, "CreateNodeItemAdapter: empty node or index over range")

	adpt, err = CreateNodeItemAdapter(NewNode(), 0)
	assert.Equal(t, NodeItemAdapter{}, adpt)
	assert.EqualError(t, err, "CreateNodeItemAdapter: empty node or index over range")

	n := NewNode()
	n.Push("hello")
	adpt, err = CreateNodeItemAdapter(n, 0)
	assert.NoError(t, err)
	assert.Equal(t, interface{}("hello"), adpt.Item(0))
	assert.Equal(t, nil, adpt.Item(1))

	adpt, err = CreateNodeItemAdapter(n, 1)
	assert.Equal(t, NodeItemAdapter{}, adpt)
	assert.EqualError(t, err, "CreateNodeItemAdapter: empty node or index over range")
}

func TestSorting(t *testing.T) {
	items := testSortItems([]uint{0, 1, 5, 55, 1, 7, 9, 100, 20})
	s := NewSorting()
	err := s.Run(items)
	assert.EqualError(t, err, "Sorting.Run: don't know how to sort: 0")

	s = NewSorting(
		WithCmpDetects(func(a interface{}) SortCmp {
			return func(a, b interface{}) bool { return a.(uint) < b.(uint) }
		}),
	)
	err = s.Run(items)
	assert.NoError(t, err)
	assert.Equal(t, []uint{0, 1, 1, 5, 7, 9, 20, 55, 100}, []uint(items))

	s = NewSorting(
		WithCmpDetects(
			func(a interface{}) SortCmp {
				return func(a, b interface{}) bool { return a.(uint) < b.(uint) }
			},
			func(a interface{}) SortCmp {
				return nil
			},
		),
	)
	err = s.Run(items)
	assert.NoError(t, err)
	assert.Equal(t, []uint{0, 1, 1, 5, 7, 9, 20, 55, 100}, []uint(items))

	s = NewSorting(
		WithDescending(),
		WithCmpDetects(func(a interface{}) SortCmp {
			return func(a, b interface{}) bool { return a.(uint) < b.(uint) }
		}),
	)
	err = s.Run(items)
	assert.NoError(t, err)
	assert.Equal(t, []uint{100, 55, 20, 9, 7, 5, 1, 1, 0}, []uint(items))
}

func TestNodeSort(t *testing.T) {
	n := NewNode()
	err := n.Sort(1)
	assert.EqualError(t, err, "Sort: no such field")

	n = NewNode()
	n.Push()
	err = n.Sort(1)
	assert.EqualError(t, err, "Sort: no such field")

	n = NewNode()
	n.Push(0, 1)
	n.Push(0)
	err = n.Sort(1)
	assert.EqualError(t, err, "Sort: not same type")

	var (
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
		}
		res     [][]interface{}
		collect = func(n *Node) {
			res = append(res, n.Row().fields)
		}
	)

	n = NewNode()
	for _, item := range data {
		n.Push(item...)
	}

	n.Sort(0)
	n.Walk(collect)
	assert.Equal(
		t,
		[][]interface{}{
			{-9, "violation", pt("1989-12-27"), "this"},
			{0, "progress", pt("1988-08-17"), "column"},
			{712, "animal", pt("1999-07-01"), "returns"},
			{712, "flawed", pt("1993-02-13"), "error"},
			{1227, "alcohol", pt("1993-02-13"), nil},
		},
		res,
	)

	res = [][]interface{}{}
	n.Sort(0, WithDescending())
	n.Walk(collect)
	assert.Equal(
		t,
		[][]interface{}{
			{1227, "alcohol", pt("1993-02-13"), nil},
			{712, "flawed", pt("1993-02-13"), "error"},
			{712, "animal", pt("1999-07-01"), "returns"},
			{0, "progress", pt("1988-08-17"), "column"},
			{-9, "violation", pt("1989-12-27"), "this"},
		},
		res,
	)

	res = [][]interface{}{}
	n.Sort(1)
	n.Walk(collect)
	assert.Equal(
		t,
		[][]interface{}{
			{1227, "alcohol", pt("1993-02-13"), nil},
			{712, "animal", pt("1999-07-01"), "returns"},
			{712, "flawed", pt("1993-02-13"), "error"},
			{0, "progress", pt("1988-08-17"), "column"},
			{-9, "violation", pt("1989-12-27"), "this"},
		},
		res,
	)

	res = [][]interface{}{}
	n.Sort(2)
	n.Walk(collect)
	assert.Equal(
		t,
		[][]interface{}{
			{0, "progress", pt("1988-08-17"), "column"},
			{-9, "violation", pt("1989-12-27"), "this"},
			{1227, "alcohol", pt("1993-02-13"), nil},
			{712, "flawed", pt("1993-02-13"), "error"},
			{712, "animal", pt("1999-07-01"), "returns"},
		},
		res,
	)

	err = n.Sort(3)
	assert.EqualError(t, err, "Sort: not same type")

	res = [][]interface{}{}
	n.Sort(0, WithCmpDetects(func(a interface{}) SortCmp {
		return func(a, b interface{}) bool { return a.(int) > b.(int) }
	}))
	n.Walk(collect)
	assert.Equal(
		t,
		[][]interface{}{
			{1227, "alcohol", pt("1993-02-13"), nil},
			{712, "flawed", pt("1993-02-13"), "error"},
			{712, "animal", pt("1999-07-01"), "returns"},
			{0, "progress", pt("1988-08-17"), "column"},
			{-9, "violation", pt("1989-12-27"), "this"},
		},
		res,
	)
}

func TestNodeTrivialTree(t *testing.T) {
	n := NewNode()
	n0, _ := n.Push()
	n1, _ := n.Push()
	n00, _ := n0.Push()
	n01, _ := n0.Push()
	n10, _ := n1.Push()
	n11, _ := n1.Push()

	assert.Equal(t, 2, n.NodesCount())
	assert.Same(t, n0, n.nodes[0])
	assert.Same(t, n1, n.nodes[1])
	assert.Same(t, n0.parent, n)
	assert.Same(t, n1.parent, n)

	assert.Equal(t, 2, n0.NodesCount())
	assert.Same(t, n00, n0.nodes[0])
	assert.Same(t, n01, n0.nodes[1])
	assert.Same(t, n00.parent, n0)
	assert.Same(t, n01.parent, n0)

	assert.Equal(t, 2, n1.NodesCount())
	assert.Same(t, n10, n1.nodes[0])
	assert.Same(t, n11, n1.nodes[1])
	assert.Same(t, n10.parent, n1)
	assert.Same(t, n11.parent, n1)
}

func TestNodePush(t *testing.T) {
	var (
		res       []string
		appendStr = func(s string) {
			res = append(res, s)
		}
	)

	n := NewNode()
	n0, err := n.Push()

	assert.NoError(t, err)
	assert.Same(t, n0.parent, n)
	assert.Same(t, n0.Schema(), n.Schema())
	assert.Equal(t, 1, n.NodesCount())
	assert.Nil(t, n.Row())
	assert.NotNil(t, n0.Row())

	n = NewNode()
	n0, _ = n.Push("1", "12", "123")
	n0.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%1s", "%2s", "%3s"}, res)

	res = []string{}
	n1, _ := n.Push()
	n1.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%1s", "%2s", "%3s"}, res)

	res = []string{}
	n2, _ := n.Push("", "123", "1234", "1234")
	n2.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%1s", "%3s", "%4s"}, res)
	assert.Same(t, n2.Schema(), n.Schema())

	res = []string{}
	n00, _ := n0.Push("12345", "12345", "12345", "12345")
	n00.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%5s", "%5s", "%5s"}, res)
	assert.Same(t, n00.Schema(), n.Schema())

	n = NewNode(WithRow(NewRow(WithColumns(NewColumn(WithWidth(20))))))
	res = []string{}
	n0, _ = n.Push("1", "12", "123")
	n0.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%20s"}, res)

	n = NewNode(WithRow(NewRow(WithData(0, 1))))
	res = []string{}
	n.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%1s", "%1s"}, res)
	assert.Equal(t, 0, n.NodesCount())

	res = []string{}
	n0, _ = n.Push("1", "12", "123")
	n0.Row().EachFmtStr(appendStr)
	assert.Equal(t, []string{"%1s", "%2s"}, res)
}

func TestNodePushNode(t *testing.T) {
	n := NewNode()
	n0, err := n.PushNode(nil)
	assert.Nil(t, n0)
	assert.EqualError(t, err, "PushNode: incoming can't be nil")

	n0, err = n.PushNode(NewNode(WithRow(nil)))
	assert.Nil(t, n0)
	assert.EqualError(t, err, "PushNode: can't add empty node")

	a := NewNode()
	b := NewNode()
	c, err := a.PushNode(b)
	assert.Nil(t, c)
	assert.EqualError(t, err, "PushNode: can't add empty node")

	a = NewNode()
	b = NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
	c, err = a.PushNode(b)
	assert.NoError(t, err)
	assert.Same(t, c, b)
	assert.Same(t, c.parent, a)
	assert.Same(t, c, a.nodes[0])

	// not supported case
	a = NewNode()
	r := NewRow()
	r.schema = nil
	b = NewNode(WithRow(r))
	c, err = a.PushNode(b)
	assert.Nil(t, c)
	assert.EqualError(t, err, "PushNode: both nodes have no schemas")

	// not supported case
	a = NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
	r = NewRow()
	r.schema = nil
	b = NewNode(WithRow(r))
	c, err = a.PushNode(b)
	assert.NoError(t, err)
	assert.Same(t, c.Schema(), a.Schema())

	a = NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
	b = NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
	c, err = a.PushNode(b)
	assert.Nil(t, c)
	assert.EqualError(t, err, "PushNode: incoming node must have the same schema")

	a = NewNode(WithRow(NewRow(WithColumns(NewColumn()))))
	b = NewNode(WithRow(NewRow(WithSchema(a.Schema()))))
	c, err = a.PushNode(b)
	assert.NoError(t, err)
	assert.Same(t, c, b)
	assert.Same(t, c.parent, a)
	assert.Same(t, c, a.nodes[0])
}

func TestNodeWalk(t *testing.T) {
	n := NewNode()
	n0, _ := n.Push(1, 1, 1)
	n.Push(2, 2, 2)
	n.Push(3, 3, 3)

	n00, _ := n0.Push(11, 11, 11, 11)
	n0.Push(12, 12)

	m := NewNode(
		WithRow(
			NewRow(
				WithSchema(n00.Schema()),
				WithData(4, 4, 4),
			),
		),
	)
	m.Push(41, 41, 41)
	m.Push(42, 42, 42)
	n.PushNode(m)

	var (
		res     []string
		doPrint = func(n *Node) {
			var str []string
			n.Row().EachFmtStr(func(s string) {
				str = append(str, s)
			})
			res = append(
				res,
				fmt.Sprintf(strings.Join(str, "|")+"\n", n.Row().FmtArgs()...),
			)
		}
	)
	n.Walk(doPrint)

	assert.Equal(
		t,
		[]string{
			" 1| 1| 1\n", "11|11|11\n", "12|12|  \n",
			" 2| 2| 2\n", " 3| 3| 3\n", " 4| 4| 4\n",
			"41|41|41\n", "42|42|42\n",
		},
		res,
	)

	res = []string{}
	m.Walk(doPrint)
	assert.Equal(t, []string{"41|41|41\n", "42|42|42\n"}, res)
}

func TestColumnString(t *testing.T) {
	c := NewColumn()
	assert.Equal(t, "%0s", c.String())

	c = NewColumn(WithWidth(-20))
	assert.Equal(t, "%0s", c.String())

	c = NewColumn(WithWidth(20))
	assert.Equal(t, "%20s", c.String())

	c = NewColumn(WithWidth(20), WithLeftAlignment())
	assert.Equal(t, "%-20s", c.String())

	c = NewColumn(WithLeftAlignment())
	assert.Equal(t, "%-0s", c.String())
}

func TestRowEachFmtStr(t *testing.T) {
	var (
		str       string
		wontReach = func(s string) {
			panic("WONT REACH")
		}
		appendStr = func(s string) {
			str += s
		}
	)

	r := NewRow()
	r.EachFmtStr(wontReach)

	r = NewRow(WithData())
	r.EachFmtStr(wontReach)

	r = NewRow(WithData(nil, -10, ""))
	str = ""
	r.EachFmtStr(appendStr)
	assert.Equal(t, "%0s%3s%0s", str)
	assert.Equal(t, []interface{}{"", "-10", ""}, r.FmtArgs())

	r = NewRow(WithColumns())
	r.EachFmtStr(wontReach)

	r = NewRow(WithColumns(), WithData())
	r.EachFmtStr(wontReach)

	r = NewRow(WithColumns(), WithData(nil, nil, nil))
	r.EachFmtStr(wontReach)

	r = NewRow(WithColumns(NewColumn(), NewColumn(), NewColumn()))
	str = ""
	r.EachFmtStr(appendStr)
	assert.Equal(t, "%0s%0s%0s", str)
	assert.Equal(t, []interface{}{"", "", ""}, r.FmtArgs())

	r = NewRow(
		WithColumns(NewColumn(), NewColumn(), NewColumn()),
		WithData(nil, -10, ""),
	)
	str = ""
	r.EachFmtStr(appendStr)
	assert.Equal(t, "%0s%3s%0s", str)
	assert.Equal(t, []interface{}{"", "-10", ""}, r.FmtArgs())

	r = NewRow(
		WithColumns(NewColumn(), NewColumn(), NewColumn()),
		WithData(nil, -10),
	)
	str = ""
	r.EachFmtStr(appendStr)
	assert.Equal(t, "%0s%3s%0s", str)
	assert.Equal(t, []interface{}{"", "-10", ""}, r.FmtArgs())

	r = NewRow(
		WithColumns(NewColumn(), NewColumn()),
		WithData(nil, -10, ""),
	)
	str = ""
	r.EachFmtStr(appendStr)
	assert.Equal(t, "%0s%3s", str)
	assert.Equal(t, []interface{}{"", "-10"}, r.FmtArgs())
}

func TestRowFmtArgs(t *testing.T) {
	r := NewRow()
	assert.Equal(t, []interface{}{}, r.FmtArgs())

	tm := time.Date(1989, 12, 27, 0, 0, 0, 0, time.UTC)

	r = NewRow(
		WithData(
			nil,
			testStruct{},
			-10,
			"",
			(*string)(nil),
			"no problem",
			tm,
			testTime(tm),
		),
	)

	assert.Equal(
		t,
		[]interface{}{
			"",
			"{}",
			"-10",
			"",
			"<nil>",
			"no problem",
			"1989-12-27 00:00:00 +0000 UTC",
			"Dec 27 1989",
		},
		r.FmtArgs(),
	)
}

func TestRowSchema(t *testing.T) {
	r := NewRow()
	assert.Equal(t, NewSchemaFrom([]interface{}{}), r.Schema())
	r = NewRow(WithData())
	assert.Equal(t, NewSchemaFrom([]interface{}{}), r.Schema())

	a := NewRow(WithData(nil, -10, ""))
	b := NewRow(WithData("", -30, ""))
	c := NewRow(WithData(0, 0, 0))

	assert.NotEqual(t, NewSchemaFrom([]interface{}{}), a.Schema())
	assert.Equal(t, b.Schema(), a.Schema())
	assert.NotEqual(t, c.Schema(), a.Schema())

	a = NewRow(
		WithColumns(NewColumn(), NewColumn()),
		WithData(0, 0, 0),
	)
	b = NewRow(WithData(9, 9))
	assert.Equal(t, b.Schema(), a.Schema())

	str := ""
	appendStr := func(s string) {
		str += s
	}
	a = NewRow(
		WithColumns(
			NewColumn(WithWidth(5)),
			NewColumn(WithWidth(5), WithLeftAlignment()),
			NewColumn(WithLeftAlignment()),
			NewColumn(),
		),
		WithData("123456", "123456", "123456", "123456"),
	)
	a.EachFmtStr(appendStr)
	assert.Equal(t, "%5s%-5s%-6s%6s", str)

	b = NewRow(
		WithSchema(a.Schema()),
		WithData("1234567890", "1234567890", "1234567890", "1234567890"),
	)
	str = ""

	assert.Same(t, b.Schema(), a.Schema())

	a.EachFmtStr(appendStr)
	assert.Equal(t, "%5s%-5s%-10s%10s", str)
}

func TestMustToString(t *testing.T) {
	tm := time.Date(1989, 12, 27, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "", MustToString(nil))
	assert.Equal(t, "{}", MustToString(testStruct{}))
	assert.Equal(t, "-10", MustToString(-10))
	assert.Equal(t, "", MustToString(""))

	assert.Equal(t, "<nil>", MustToString((*string)(nil)))

	assert.Equal(t, "no problem", MustToString("no problem"))
	assert.Equal(t, "1989-12-27 00:00:00 +0000 UTC", MustToString(tm))
	assert.Equal(t, "Dec 27 1989", MustToString(testTime(tm)))
}

type testStruct struct{}
type testTime time.Time

func (t testTime) String() string {
	return time.Time(t).Format("Jan _2 2006")
}

type testInt int

func (t testInt) LessThan(a interface{}) bool {
	// implement greater than to test
	return int(t) > int(a.(testInt))
}

type testSortItems []uint

func (t testSortItems) Items() interface{} {
	return interface{}(t)
}

func (t testSortItems) Item(i int) interface{} {
	return interface{}(t[i])
}
