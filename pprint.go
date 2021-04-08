package pprint

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Node struct {
	parent *Node
	nodes  []*Node
	schema *ColumnSchema
	row    *Row
}

func (n *Node) Push(a ...interface{}) (newNode *Node, err error) {
	return n.PushRow(
		NewRow(WithSchema(n.schema), WithData(a...)),
	)
}

func (n *Node) PushRow(r *Row) (newNode *Node, err error) {
	return n.PushNode(NewNode(WithRow(r)))
}

// cannot add if:
// 1. current_node.schema != add_node.raw.schema
// 2. current_node.schema == nil && add_node.raw.schema == nil  // nonsense
// takes add_node.raw.schema
// BUG: loop
func (n *Node) PushNode(incoming *Node) (modified *Node, err error) {
	if incoming == nil {
		return nil, fmt.Errorf("PushNode: incoming can't be nil")
	}

	ir := incoming.Row()
	if ir == nil {
		return nil, fmt.Errorf("PushNode: can't add empty node")
	}

	irs := ir.Schema()
	if irs == nil {
		if n.schema == nil {
			return nil, fmt.Errorf("PushNode: both nodes have no schemas")
		}
		incoming.schema = n.schema
	} else if n.schema == nil {
		n.schema = irs
	} else if n.schema != irs {
		return nil, fmt.Errorf("PushNode: incoming node must have the same schema")
	}

	incoming.parent = n
	n.nodes = append(n.nodes, incoming)

	return incoming, err

}

func (n *Node) Sort(col int, opts ...SortingOpt) error {
	if n.schema == nil || col < 0 || col >= n.schema.count {
		return fmt.Errorf("Sort: no such field")
	}
	if n.NodesCount() < 2 {
		return nil
	}

	cell := func(row int) interface{} {
		return n.nodes[row].row.fields[col]
	}

	for i, j := 0, 1; j < n.NodesCount(); i, j = i+1, j+1 {
		if reflect.TypeOf(cell(i)) != reflect.TypeOf(cell(j)) {
			return fmt.Errorf("Sort: not same type")
		}
	}

	adpt, _ := CreateNodeItemAdapter(n, col)

	return NewSorting(opts...).Run(adpt)
}

func (n *Node) Row() *Row {
	return n.row
}

func (n *Node) Walk(fn func(*Node)) {
	n.EachNode(func(c *Node) {
		fn(c)
		c.Walk(fn)
	})
}

func (n *Node) EachNode(fn func(*Node)) {
	for _, c := range n.nodes {
		fn(c)
	}
}

func (n *Node) NodesCount() int {
	return len(n.nodes)
}

func (n *Node) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithSeparator(" ")).RunNode(n)
	return b.String()
}

func (n *Node) Parent() *Node {
	return n.parent
}

func (n *Node) IsNotRoot() bool {
	return n.Parent() != nil
}

func (n *Node) Schema() *ColumnSchema {
	return n.schema
}

func NewNode(opts ...NodeOpt) *Node {
	n := &Node{}
	for _, opt := range opts {
		opt(n)
	}
	if n.row != nil {
		n.schema = n.row.Schema()
	}
	return n
}

type NodeOpt func(*Node)

func WithRow(r *Row) NodeOpt {
	return func(n *Node) {
		n.row = r
	}
}

type NodeItemAdapter struct {
	nodes  []*Node
	column int
}

func (adpt NodeItemAdapter) Items() interface{} {
	return interface{}(adpt.nodes)
}

func (adpt NodeItemAdapter) Item(row int) interface{} {
	if row < 0 || row >= len(adpt.nodes) {
		return nil
	}
	return adpt.nodes[row].row.fields[adpt.column]
}

func CreateNodeItemAdapter(n *Node, col int) (NodeItemAdapter, error) {
	if n == nil || n.schema == nil || col < 0 || col >= n.schema.count {
		return NodeItemAdapter{}, fmt.Errorf("CreateNodeItemAdapter: empty node or index over range")
	}
	adpt := NodeItemAdapter{
		nodes:  n.nodes,
		column: col,
	}
	return adpt, nil
}

type Column struct {
	width int
	pad   struct {
		fixed bool
		right bool
	}
}

func (c Column) String() string {
	if s := strconv.FormatInt(int64(c.width), 10); c.pad.right {
		return "%-" + s + "s"
	} else {
		return "%" + s + "s"
	}
}

func NewColumn(opts ...ColumnOpt) Column {
	c := Column{}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type ColumnOpt func(*Column)

func WithWidth(w int) ColumnOpt {
	return func(c *Column) {
		if w < 0 {
			w = 0
		}
		c.width = w
		c.pad.fixed = true
	}
}

func WithLeftAlignment() ColumnOpt {
	return func(c *Column) {
		c.pad.right = true
	}
}

type ColumnSchema struct {
	cols  []Column
	count int
}

func NewSchema(c ...Column) *ColumnSchema {
	return &ColumnSchema{
		cols:  c,
		count: len(c),
	}
}

func NewSchemaFrom(fields []interface{}) *ColumnSchema {
	size := len(fields)

	return &ColumnSchema{
		cols:  make([]Column, size),
		count: size,
	}
}

type Row struct {
	schema  *ColumnSchema
	fields  []interface{}
	fmtArgs []interface{}
}

func (r *Row) EachFmtStr(fn func(string)) {
	for _, c := range r.schema.cols {
		fn(c.String())
	}
}

func (r *Row) FmtArgs() []interface{} {
	return r.fmtArgs
}

func (r *Row) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithSeparator(" "), WithoutLf()).RunRow(r)
	return b.String()
}

func (r *Row) Schema() *ColumnSchema {
	return r.schema
}

func (r *Row) prepare() {
	if f := r.fields; r.schema == nil {
		r.schema = NewSchemaFrom(f)
	} else {
		// apply schema: make same fields count
		r.fields = resizeSlice(f, r.schema.count)
	}

	r.fmtArgs = make([]interface{}, r.schema.count)

	for i := 0; i < r.schema.count; i++ {
		r.fmtArgs[i] = MustToString(r.fields[i])

		if c := r.schema.cols[i]; !c.pad.fixed {
			w := len(r.fmtArgs[i].(string))
			if w > c.width {
				r.schema.cols[i].width = w
			}
		}
	}
}

func NewRow(opts ...RowOpt) *Row {
	r := &Row{}
	for _, opt := range opts {
		opt(r)
	}
	r.prepare()
	return r
}

type RowOpt func(*Row)

func WithSchema(s *ColumnSchema) RowOpt {
	return func(r *Row) {
		r.schema = s
	}
}

func WithColumns(c ...Column) RowOpt {
	return func(r *Row) {
		r.schema = NewSchema(c...)
	}
}

func WithData(a ...interface{}) RowOpt {
	return func(r *Row) {
		r.fields = a
	}
}

type SortItems interface {
	Items() interface{}
	Item(i int) interface{}
}
type SortCmp func(a, b interface{}) bool

type Sorting struct {
	cmps [](func(interface{}) SortCmp)
	desc bool
}

func (s *Sorting) Run(data SortItems) error {
	cmp := s.runCmpChain(data.Item(0))
	if cmp == nil {
		return fmt.Errorf("Sorting.Run: don't know how to sort: %v", data.Item(0))
	}

	var less func(i, j int) bool
	if s.desc {
		less = func(i, j int) bool { return !cmp(data.Item(i), data.Item(j)) }
	} else {
		less = func(i, j int) bool { return cmp(data.Item(i), data.Item(j)) }
	}

	sort.SliceStable(data.Items(), less)

	return nil
}

func (s *Sorting) runCmpChain(a interface{}) SortCmp {
	for i := len(s.cmps) - 1; i >= 0; i-- {
		cmp := s.cmps[i](a)
		if cmp != nil {
			return cmp
		}
	}
	return nil
}

func NewSorting(opts ...SortingOpt) *Sorting {
	s := &Sorting{}
	s.cmps = append(s.cmps, DetectCmp)
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type SortingOpt func(*Sorting)

func WithDescending() SortingOpt {
	return func(s *Sorting) {
		s.desc = true
	}
}

func WithCmpDetects(cmps ...func(interface{}) SortCmp) SortingOpt {
	return func(s *Sorting) {
		s.cmps = append(s.cmps, cmps...)
	}
}

type Printing struct {
	w   io.Writer
	lf  string
	sep string
}

func (p *Printing) RunNode(n *Node) {
	if n == nil {
		return
	}

	if n.IsNotRoot() {
		p.RunRow(n.Row())
	}

	n.Walk(func(n *Node) {
		p.RunRow(n.Row())
	})
}

func (p *Printing) RunRow(r *Row) {
	if r == nil {
		return
	}

	str := ""
	r.EachFmtStr(func(s string) {
		str += p.sep
		str += s
	})
	if str == "" {
		return
	} else if p.sep != "" {
		str = str[len(p.sep):]
	}

	if p.lf != "" {
		str += p.lf
	}

	fmt.Fprintf(p.w, str, r.FmtArgs()...)
}

func NewPrinting(opts ...PrintingOpt) *Printing {
	p := &Printing{
		w:   os.Stdout,
		lf:  "\n",
		sep: " ",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func Print(n *Node, opts ...PrintingOpt) {
	NewPrinting(opts...).RunNode(n)
}

type PrintingOpt func(*Printing)

func WithSeparator(sep string) PrintingOpt {
	return func(p *Printing) {
		p.sep = sep
	}
}

func WithWriter(w io.Writer) PrintingOpt {
	return func(p *Printing) {
		p.w = w
	}
}

func WithoutLf() PrintingOpt {
	return func(p *Printing) {
		p.lf = ""
	}
}

func DetectCmp(a interface{}) SortCmp {
	var out SortCmp

	switch a.(type) {
	case string:
		out = func(a, b interface{}) bool { return a.(string) < b.(string) }
	case int:
		out = func(a, b interface{}) bool { return a.(int) < b.(int) }
	case time.Time:
		out = func(a, b interface{}) bool { return a.(time.Time).Before(b.(time.Time)) }
	}

	return out
}

func resizeSlice(s []interface{}, become int) []interface{} {
	switch cur := len(s); {
	case cur == become:
	case cur < become:
		s = append(s, make([]interface{}, become-cur)...)
	case cur > become:
		s = s[0:become]
	}
	return s
}

// Converts anything to a string.
// The function itself handles the common types including: fmt.Stringer, string, []byte, uint, int and nil.
// It passes anything else to the fmt.Sprintf to get the string representation of that value.
func MustToString(a interface{}) string {
	var s string

	switch v := a.(type) {
	case fmt.Stringer:
		s = v.String()
	case string:
		s = v
	case []byte:
		s = string(v)
	case uint:
		s = strconv.FormatUint(uint64(v), 10)
	case int:
		s = strconv.FormatInt(int64(v), 10)
	case nil:
	default:
		s = fmt.Sprintf("%v", v)
	}

	return s
}
