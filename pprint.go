// Package pprint is a library to typeset output with auto-width padding. It's implemented using a tree and has directory-like context. Supports typesetting or sorting per directory context.
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

type nodes []*Node

func (n nodes) cell(col, row int) interface{} {
	return n[row].Row().fields[col]
}

// Implements a tree node
type Node struct {
	nodes
	parent *Node

	// Defines how many columns a row contains for all its child nodes.
	schema *ColumnSchema

	// Row data goes here. It's constrained by parent's schema.
	row *Row
}

// All the input will be wrapped into a new node, and will become a child of the node being pushed to.
// Returns a pointer to the newly created node. The schema of the newly created node will be set to the same as the node
// being pushed to, i.e. it inherits the schema.
//
// Therefore if we're pushing a row containing 5 fields, but the current node only have 4 in its schema, the 5th field will
// be silently discarded. If there are 6 in its schema, an empty field will be generated automatically.
func (n *Node) Push(a ...interface{}) (newNode *Node, err error) {
	return n.PushRow(
		NewRow(WithSchema(n.schema), WithData(a...)),
	)
}

// The method can be used to push data with different schema to a leaf node.
func (n *Node) PushRow(r *Row) (newNode *Node, err error) {
	return n.PushNode(NewNode(WithRow(r)))
}

// Merges the node into the tree by placing incoming into the child nodes of the receiving node.
// Returns the pointer to the pushed node.
//
// 1) The row in an incoming node cannot be nil.
//
// 2) The schemas of both nodes cannot be nil.
//
// 3) Since receiving's schema dominates its children, the incoming's schema must be identical to receiving's.
//
// 4) If the schema in either node is nil, it will be set to that of the other's, to implement inheritance.
//
// In other words, if A is a leaf node, any B can be pushed as long as it's not empty. Otherwise, A accepts B only if
// they share the same schema object.
//
// BUG(adios): I didn't implement loop detection. Better to use this function only if you understand what you need.
func (n *Node) PushNode(incoming *Node) (modified *Node, err error) {
	if incoming == nil {
		return nil, fmt.Errorf("PushNode: incoming can't be nil")
	}

	ir := incoming.Row()
	if ir == nil {
		// case 1: no sense if there is no data to add
		return nil, fmt.Errorf("PushNode: can't add empty node")
	}

	irs := ir.Schema()
	if irs == nil {
		// by current design, there are no ways to enter this branch, irs won't be nil.
		if n.schema == nil {
			return nil, fmt.Errorf("PushNode: both nodes have no schemas")
		}
		incoming.schema = n.schema
	} else if n.schema == nil {
		// case 4: inheritance
		n.schema = irs
	} else if n.schema != irs {
		// case 3: reject if not the same
		return nil, fmt.Errorf("PushNode: incoming node must have the same schema")
	}

	incoming.parent = n
	n.nodes = append(n.nodes, incoming)

	return incoming, err
}

// Sort the node's children (rows) based on the original value stored on the given column index, index starts from 0.
// Sorting options are:
//
// WithDescending(): default is to sort in ascending.
//
// WithCmpMatchers(fn): set fn to sort additional types. By default Sort() can handle: int, string and time.Time.
//
// Note that it sorts with stable algorithm, and sorts on original value, not the outputting string represention. It sort
// only current children, not recursively to its all descendants.
func (n *Node) Sort(col int, opts ...SortOpt) error {
	if n.schema == nil || col < 0 || col >= n.schema.count {
		return fmt.Errorf("Sort: no such field")
	}
	if n.NodesCount() < 2 {
		return nil
	}

	nodes, err := createSortableOn(col, []*Node(n.nodes), opts...)
	if err != nil {
		return err
	}

	sort.Stable(nodes)
	return nil
}

// Do things on each descendant of current node.
func (n *Node) Walk(fn func(*Node)) {
	n.EachNode(func(c *Node) {
		fn(c)
		c.Walk(fn)
	})
}

// Do things on each child of current node. Use Walk to do things on all descendants.
func (n *Node) EachNode(fn func(*Node)) {
	for _, c := range n.nodes {
		fn(c)
	}
}

// Returns the final output (includes all descendants) in a string with default printing options.
func (n *Node) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithColSep(" ")).RunNode(n)
	return b.String()
}

// Returns how many children this node has.
func (n *Node) NodesCount() int {
	return len(n.nodes)
}

func (n *Node) Parent() *Node {
	return n.parent
}

// Returns the attached row in current node. And normally, a root node has no row attached.
func (n *Node) Row() *Row {
	return n.row
}

// Returns the schema that is used to donminate the node's children.
func (n *Node) Schema() *ColumnSchema {
	return n.schema
}

// Check if a node isn't a tree root.
func (n *Node) IsNotRoot() bool {
	return n.Parent() != nil
}

// Options:
//
// WithRow(*Row): attaches the *Row instance to the newly created node. Used to manually create a non-root node.
func NewNode(opts ...NodeOpt) *Node {
	n := &Node{}
	for _, opt := range opts {
		opt(n)
	}
	if n.row != nil {
		// A NewRow() created row always has a schema instance.
		n.schema = n.row.Schema()
	}
	return n
}

type NodeOpt func(*Node)

// WithRow(*Row): attaches the *Row instance to the newly created node. Used to manually create a non-root node.
func WithRow(r *Row) NodeOpt {
	return func(n *Node) {
		n.row = r
	}
}

// Column relates to padding. It stores the width a column should be at current time.
type Column struct {
	width int
	pad   struct {
		fixed bool
		right bool
	}
}

// Returns the FmtStr representation of the current column to be used in the fmt.Printf. e.g.: "%3s", "%-5s".
func (c Column) String() string {
	if s := strconv.FormatInt(int64(c.width), 10); c.pad.right {
		return "%-" + s + "s"
	} else {
		return "%" + s + "s"
	}
}

// Options are:
//
// WithWidth(int): set to disable auto padding on this column. Always output with the given width of padding. E.g.:
// WithWidth(20) equals to "%20s".
//
// WithLeftAlignment(): set to pad to the right. E.g.: WithWidth(20), WithLeftAlignment() = "%-20s".
func NewColumn(opts ...ColumnOpt) Column {
	c := Column{}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type ColumnOpt func(*Column)

// WithWidth(int): set to disable auto padding on this column. Always output with the given width of padding. E.g.:
// WithWidth(20) equals to "%20s".
func WithWidth(w int) ColumnOpt {
	return func(c *Column) {
		if w < 0 {
			w = 0
		}
		c.width = w
		c.pad.fixed = true
	}
}

// WithLeftAlignment(): set to pad to the right. E.g.: WithWidth(20), WithLeftAlignment() = "%-20s".
func WithLeftAlignment() ColumnOpt {
	return func(c *Column) {
		c.pad.right = true
	}
}

// How many columns in a row.
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

// Automatically generate the corresponding Columns on given fields.
func NewSchemaFrom(fields []interface{}) *ColumnSchema {
	size := len(fields)

	return &ColumnSchema{
		cols:  make([]Column, size),
		count: size,
	}
}

// The struct that saves both the original input and the converted string output.
type Row struct {

	// Defines how many columns this row contains.
	schema *ColumnSchema

	// Stores original unmodified inputs that is shrinked or expanded to alignto the schema.
	fields []interface{}

	// Stores the string representation of each input and is shrinked or expanded to align to the schema.
	// The data is also required to calculate column widths.
	// Use []interface{} instead of []string so that I can pass to fmt.Printf without creating a new slice.
	fmtArgs []interface{}
}

// Do things on each FmtStr string from the columns defined in the attached ColumnSchema instance.
func (r *Row) EachFmtStr(fn func(string)) {
	for _, c := range r.schema.cols {
		fn(c.String())
	}
}

// Returns a string slice that stores the string representation of each input field that is shrinked or expanded to align
// to the schema.
func (r *Row) FmtArgs() []interface{} {
	return r.fmtArgs
}

// Returns the row's final output in a string with default printing options.
func (r *Row) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithColSep(" "), WithLineBrk("")).RunRow(r)
	return b.String()
}

func (r *Row) Schema() *ColumnSchema {
	return r.schema
}

// Preparation the following things:
// 1. maintains the schema, creates one if it has no schema.
// 2. shrinks or expands input fields to align to the schema.
// 3. convert input to string, updates its length into the schema instance.
func (r *Row) prepare() {
	if f := r.fields; r.schema == nil {
		r.schema = NewSchemaFrom(f)
	} else {
		// shrink or expand fields
		r.fields = resizeSlice(f, r.schema.count)
	}

	r.fmtArgs = make([]interface{}, r.schema.count)

	for i := 0; i < r.schema.count; i++ {
		r.fmtArgs[i] = MustToString(r.fields[i])

		if c := r.schema.cols[i]; !c.pad.fixed {
			// only updates to those without fixed width
			w := len(r.fmtArgs[i].(string))
			if w > c.width {
				r.schema.cols[i].width = w
			}
		}
	}
}

// A created row will always have schema instance. Options are:
//
// WithSchema(*ColumnSchema): to inherit the schema from an existing row or node.
//
// WithColumns(...Column): to create a row with fixed-width, left-alignment columns.
//
// WithData(...interface{}): anything to be set in the row.
//
// If no schema nor columns options specified, a row will generate the schema based on the input data and auto-width
// are always applyed to the auto-generated columns.
func NewRow(opts ...RowOpt) *Row {
	r := &Row{}
	for _, opt := range opts {
		opt(r)
	}
	r.prepare()
	return r
}

type RowOpt func(*Row)

// WithSchema(*ColumnSchema): to inherit the schema from an existing row or node.
func WithSchema(s *ColumnSchema) RowOpt {
	return func(r *Row) {
		r.schema = s
	}
}

// WithColumns(...Column): to create a row with fixed-width, left-alignment columns.
func WithColumns(c ...Column) RowOpt {
	return func(r *Row) {
		r.schema = NewSchema(c...)
	}
}

// WithData(...interface{}): anything to be set in the row.
func WithData(a ...interface{}) RowOpt {
	return func(r *Row) {
		r.fields = a
	}
}

// Converts anything to a string. The function itself handles the common types including:
// fmt.Stringer, string, []byte, uint, int and nil. It passes anything else to the fmt.Sprintf
// to get the string representation of that value.
//
// The function is used while constructing a Row instance.
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

// To cut or to expand input fields.
func resizeSlice(s []interface{}, become int) []interface{} {
	switch cur := len(s); {
	case cur == become:
	case cur < become:
		// Newly expanded fields are set to nil.
		s = append(s, make([]interface{}, become-cur)...)
	case cur > become:
		s = s[0:become]
	}
	return s
}

// A function that takes any two values of a same type and can returns a boolean.
type CmpFn func(a, b interface{}) bool

// Less() in sort.Interface
type lessFn func(i, j int) bool

// An adapter to transform Node.nodes so that it can be sort via sort.Stable.
type sortable struct {
	nodes

	// positions the x on this column, leaves y variant, i.e. to compare value on this field.
	col int

	count int

	// sort in descending order
	desc bool

	less lessFn

	// a chain of func that can generate some CmpFn,
	// we run through this chain to find one CmpFn that can handle the type we are sorting.
	chain []func(a interface{}) CmpFn
}

// Find a CmpFn that is able to handle (do comparison on) type of a.
func (s *sortable) matchComparator(a interface{}) (cmp CmpFn, ok bool) {
	for _, matcher := range s.chain {
		cmp := matcher(a)
		if cmp != nil {
			return cmp, true
		}
	}
	return nil, false
}

func (s *sortable) holdsIdenticalType() bool {
	switch {
	case s.count < 2:
	case s.count >= 2:
		for i, j := 0, 1; j < s.count; i, j = i+1, j+1 {
			if reflect.TypeOf(s.cell(i)) != reflect.TypeOf(s.cell(j)) {
				return false
			}
		}
	}
	return true
}

func (s *sortable) toLess(cmp CmpFn) lessFn {
	if s.desc {
		return func(i, j int) bool { return !cmp(s.cell(i), s.cell(j)) }
	} else {
		return func(i, j int) bool { return cmp(s.cell(i), s.cell(j)) }
	}
}

// Cell retrieving method on nth column
func (s *sortable) cell(row int) interface{} {
	return s.nodes.cell(s.col, row)
}

func (s *sortable) Len() int {
	return s.count
}

func (s *sortable) Swap(i, j int) {
	s.nodes[j], s.nodes[i] = s.nodes[i], s.nodes[j]
}

func (s *sortable) Less(i, j int) bool {
	return s.less(i, j)
}

// []*Node is a 2-dimensions table. To be able to sort []*Node (rows), we need:
//
// 1. in which column, the field value to be taken.
// 2. check if all the field values are in the same type of that column.
// 3. if yes, we need to set up a Less() of that type.
func createSortableOn(column int, ns []*Node, opts ...SortOpt) (*sortable, error) {
	s := &sortable{
		nodes: nodes(ns),
		col:   column,
		count: len(ns),

		// set up a fallback Less(), so that a incidental call to Less() won't panic on empty Sortable.
		less: func(i, j int) bool { return true },
	}
	for _, opt := range opts {
		opt(s)
	}
	// put the default CmpFn finder that can compare types of string, int and time.Time.
	s.chain = append(s.chain, MatchCmp)

	if s.count > 0 {
		if !s.holdsIdenticalType() {
			return nil, fmt.Errorf("createSortableOn: column %d doesn't contain identical value type", column)
		}

		cmp, ok := s.matchComparator(s.cell(0))
		if !ok {
			return nil, fmt.Errorf("createSortableOn: don't know how to sort %s", reflect.TypeOf(s.cell(0)))
		}
		s.less = s.toLess(cmp)
	}

	return s, nil
}

type SortOpt func(*sortable)

func WithDescending() SortOpt {
	return func(s *sortable) {
		s.desc = true
	}
}

// Multiple matcher functions can be provided as input.
// They will be executed in order until a matcher can handle the current comparing type.
// See MatchCmp() to learn how to write a matcher.
func WithCmpMatchers(m ...func(interface{}) CmpFn) SortOpt {
	return func(s *sortable) {
		s.chain = append(s.chain, m...)
	}
}

// The default CmpFn matcher used in createSortableOn(). It uses type switch to find the type it can compare.
// If you know the type of the comparing field, you could simply return a CmpFn without type switching.
func MatchCmp(a interface{}) CmpFn {
	var out CmpFn
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

// Represents the printing algorithms.
type Printing struct {
	writer    io.Writer
	colSep    string
	colSepLen int
	lineBrk   string
}

// Do nothing if n is nil.
func (p *Printing) RunNode(n *Node) {
	if n == nil {
		return
	}
	if n.IsNotRoot() {
		// only root has no *Row
		p.RunRow(n.Row())
	}
	n.Walk(func(n *Node) {
		p.RunRow(n.Row())
	})
}

// Do nothing if r is nil or there is no columns to print.
func (p *Printing) RunRow(r *Row) {
	if r == nil {
		return
	}

	str := ""
	r.EachFmtStr(func(s string) {
		str += p.colSep
		str += s
	})

	if str == "" {
		// means no columns to print, will panic fmt.Printf if r.FmtArgs() isn't nil
		return
	}

	if p.colSepLen > 0 {
		str = str[p.colSepLen:]
	}
	if p.lineBrk != "" {
		str += p.lineBrk
	}

	fmt.Fprintf(p.writer, str, r.FmtArgs()...)
}

// Options are:
//
// WithColSep(string): set column separator (field separator). Defaults to " ".
//
// WithLineBrk(string): set line break. Defaults to "\n".
//
// WithWriter(io.Writer): set writer. Defaults to os.Stdout.
func NewPrinting(opts ...PrintingOpt) *Printing {
	p := &Printing{
		writer:  os.Stdout,
		colSep:  " ",
		lineBrk: "\n",
	}
	for _, opt := range opts {
		opt(p)
	}
	p.colSepLen = len(p.colSep)

	return p
}

// A convenient helper to run a Printing instance.
func Print(n *Node, opts ...PrintingOpt) {
	NewPrinting(opts...).RunNode(n)
}

type PrintingOpt func(*Printing)

// WithColSep(string): set column separator (field separator). Defaults to " ".
func WithColSep(sep string) PrintingOpt {
	return func(p *Printing) {
		p.colSep = sep
	}
}

// WithLineBrk(string): set line break. Defaults to "\n".
func WithLineBrk(brk string) PrintingOpt {
	return func(p *Printing) {
		p.lineBrk = brk
	}
}

// WithWriter(io.Writer): set writer. Defaults to os.Stdout.
func WithWriter(w io.Writer) PrintingOpt {
	return func(p *Printing) {
		p.writer = w
	}
}
