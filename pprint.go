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

// Tree node.
type Node struct {
	nodes
	parent *Node

	// Schema on child nodes.
	schema *ColumnSchema

	// All input data goes here. The schema must be identical with parent's schema.
	// For root nodes, this field could be nil.
	row *Row
}

// Creates a node to store the inputs and makes it a child of the current receiver.
// Returns a pointer to the created node and any error encountered.
//
// The parent of the receiver applys its schema to the inputs being pushed.
// That is, all the column numbers, width and padding of the inputs inherits the schema that is shared among entire tree.
//
// If the reciever has no schema, i.e. a root node without children.
// A schema instance will be generated based on the first pushed input.
// The self-generated schema instance is always of columns with auto-width and right-alignment.
//
// Use PushRow() or PushNode() to create tree containing different schemas per node.
//
// The column (fields) amount of the input doesn't have to be the same with receiver's.
// It will be enlarged (with empty string) or shrinked to fit the receiver's schema.
func (n *Node) Push(a ...interface{}) (newNode *Node, err error) {
	var opts []RowOpt

	switch n.schema == nil {
	case true:
		// Receiver has no children, it's ok to accept any new nodes,
		if n.parent == nil {
			opts = []RowOpt{WithRowData(a...)}
		} else {
			// but we force it to inherit by giving my parent's schema
			opts = []RowOpt{WithRowSchema(n.parent.schema), WithRowData(a...)}
		}
	case false:
		// Receiver has children, we new a Row with identical schema to enforce inheritance.
		opts = []RowOpt{WithRowSchema(n.schema), WithRowData(a...)}
	}
	return n.PushRow(NewRow(opts...))
}

// Accepts a customized Row. Returns a pointer to the created node and any error encountered.
func (n *Node) PushRow(r *Row) (newNode *Node, err error) {
	return n.PushNode(NewNode(WithRow(r)))
}

// Makes incoming node become a child of the receiver. Returns a pointer to the mutated incoming node and
// any error encountered.
//
// Consistency is maintained by comparing each other's node.schema and node.row.schema.
// Receiver(A) accepts incoming node(B) if:
//
// 1. A has no node schema, B contains a Row instance.
// 2. A has no node schema, B contains no Row instance, but B has node schema (tree root)
// 3. A has node schema, B contains no Row instance. (tree root)
// 4. A has node schema, B contains a Row instance with the schema which is exactly A's node schema.
//
// BUG(adios): Use carefully, no loop detections.
func (n *Node) PushNode(in *Node) (inMutated *Node, err error) {
	if in == nil {
		return nil, fmt.Errorf("PushNode: nil incoming")
	}

	switch n.schema == nil {
	case true:
		// A has no schema, it's open, try to get one by searching B
		switch in.Row() == nil {
		case true:
			if in.Schema() == nil {
				return nil, fmt.Errorf("PushNode: no schema to set")
			}
			// B is a tree root, promotes B's node schema to my schema, and gives B a Row to merge.
			n.schema = in.Schema()
			in.row = NewRow(WithRowSchema(n.schema))
		case false:
			// B has row, promotes it to become my schema
			n.schema = in.Row().Schema()
		}
	case false:
		switch in.Row() == nil {
		case true:
			// B is a tree root without Row instance, gives it an empty Row to merge.
			in.row = NewRow(WithRowSchema(n.schema))
		case false:
			if in.Row().Schema() != n.schema {
				return nil, fmt.Errorf("PushNode: row of the incoming node doesn't match my node schema")
			}
			// Same schema is allowed
		}
	}

	in.parent = n
	n.nodes = append(n.nodes, in)

	return in, err
}

// Sort receiver's child nodes (that contain rows) on the given column of that node's row.
// Accepts a column index starting from 0. Returns any error encountered.
//
// It uses stable sort to compare the raw value of the specified column field.
// Sort on values with non identical type returns an error.
// Sort on values with no type comparators returns an error.
//
// Note that it doesn't sort descendants.
//
// Sorting options are:
//
// WithDescending(): default is ascending.
//
// WithCmpMatchers(...func(a interface{}) CmpFn): to sort more types. Builtins: int, string and time.Time.
func (n *Node) Sort(col int, opts ...SortOpt) error {
	if n.schema == nil || col < 0 || col >= n.schema.count {
		return fmt.Errorf("Sort: column %d doesn't exist", col)
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

// Traverses receiver's descendants.
func (n *Node) Walk(fn func(*Node)) {
	n.EachNode(func(c *Node) {
		fn(c)
		c.Walk(fn)
	})
}

// Traverses receiver's children. Use Walk() to traverse descendants.
func (n *Node) EachNode(fn func(*Node)) {
	for _, c := range n.nodes {
		fn(c)
	}
}

// Collects each descendant's String() and prints with default options.
func (n *Node) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithColSep(" ")).RunNode(n)
	return b.String()
}

// Returns receiver's child count.
func (n *Node) NodesCount() int {
	return len(n.nodes)
}

// Returns receiver's parent.
func (n *Node) Parent() *Node {
	return n.parent
}

// Returns the attached Row instance of current receiver.
func (n *Node) Row() *Row {
	return n.row
}

// Returns the schema instance of current receiver.
func (n *Node) Schema() *ColumnSchema {
	return n.schema
}

// Returns true if receiver has parent.
func (n *Node) IsNotRoot() bool {
	return n.Parent() != nil
}

// Returns a pointer to a Node instance. Node options are:
//
// WithRow(*Row): creates a node with provided Row instance.
//
// WithSchema(*ColumnSchema): to inherit the schema from an existing row or node to be applied to all of its children.
//
// WithColumns(...Column): to create a node with provided column schema to be applied to all of its children.
func NewNode(opts ...NodeOpt) *Node {
	n := &Node{}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

type NodeOpt func(*Node)

// To inherit the schema from an existing row or node to be applied to all of its children.
func WithSchema(s *ColumnSchema) NodeOpt {
	return func(n *Node) {
		n.schema = s
	}
}

// To create a node with provided column schema to be applied to all of its children.
func WithColumns(c ...Column) NodeOpt {
	return func(n *Node) {
		n.schema = NewSchema(c...)
	}
}

// Creates a node with provided Row instance.
func WithRow(r *Row) NodeOpt {
	return func(n *Node) {
		n.row = r
	}
}

// Stores alignment and width.
type Column struct {
	width int
	pad   struct {
		fixed bool
		right bool
	}
}

// Turns current column into a format string, e.g.: "%3s", "%-5s".
func (c Column) String() string {
	if s := strconv.FormatInt(int64(c.width), 10); c.pad.right {
		return "%-" + s + "s"
	} else {
		return "%" + s + "s"
	}
}

// Returns a Column instance. Column options are:
//
// WithWidth(int): by default all columns are auto-width. Set to fix-width. WithWidth(20) is translated to "%20s".
//
// WithLeftAlignment(): set to pad to the right. For example: WithWidth(20), WithLeftAlignment() = "%-20s".
func NewColumn(opts ...ColumnOpt) Column {
	c := Column{}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type ColumnOpt func(*Column)

// By default all columns are auto-width. Set to fix-width. WithWidth(20) is translated to "%20s".
func WithWidth(w int) ColumnOpt {
	return func(c *Column) {
		if w < 0 {
			w = 0
		}
		c.width = w
		c.pad.fixed = true
	}
}

// Set to pad to the right. For example: WithWidth(20), WithLeftAlignment() = "%-20s".
func WithLeftAlignment() ColumnOpt {
	return func(c *Column) {
		c.pad.right = true
	}
}

// Defines how many columns in a row and their corresponding Column data.
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

// Creates a column schema instance with N columns. N is the length of input fields.
func NewSchemaFrom(fields []interface{}) *ColumnSchema {
	size := len(fields)

	return &ColumnSchema{
		cols:  make([]Column, size),
		count: size,
	}
}

// Stores both raw input values and their string representations.
type Row struct {

	// Defines how many columns this row contains.
	schema *ColumnSchema

	// Raw input values, has been shrinked or enlarged by current column schema.
	fields []interface{}

	// String representations of Row.fields. Used to calculate padding and fmt.Printf().
	fmtArgs []interface{}
}

// Traverses format strings with String() on each Column instance.
func (r *Row) EachFmtStr(fn func(string)) {
	for _, c := range r.schema.cols {
		fn(c.String())
	}
}

// Returns the string slice that stores the string representations of raw values.
func (r *Row) FmtArgs() []interface{} {
	return r.fmtArgs
}

// Returns a string by calling fmt.Fprintf() on fmtStr and fmtArgs.
func (r *Row) String() string {
	var b strings.Builder
	NewPrinting(WithWriter(&b), WithColSep(" "), WithLineBrk("")).RunRow(r)
	return b.String()
}

func (r *Row) Schema() *ColumnSchema {
	return r.schema
}

// Initializes a Row instance, on each creation:
//
// 1. if no schema found, create a new one based on current data.
// 2. if with schema, shrink or enlarge input fields to fit to the schema.
// 3. do string conversion, calculate string length, updates to schema instance.
func (r *Row) prepare() {
	switch fs := r.fields; r.schema == nil {
	case true:
		// auto creation
		r.schema = NewSchemaFrom(fs)
	case false:
		// shrink or enlarge fields
		r.fields = resizeSlice(fs, r.schema.count)
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

// Returns a pointer to a Row instance. Row options are:
//
// WithRowSchema(*ColumnSchema): to inherit the schema from an existing row or node.
//
// WithRowColumns(...Column): to create a row with provided column schema.
//
// WithData(...interface{}): set data to the row.
func NewRow(opts ...RowOpt) *Row {
	r := &Row{}
	for _, opt := range opts {
		opt(r)
	}
	r.prepare()
	return r
}

type RowOpt func(*Row)

// To inherit the schema from an existing row or node.
func WithRowSchema(s *ColumnSchema) RowOpt {
	return func(r *Row) {
		r.schema = s
	}
}

// To create a row with provided column schema.
func WithRowColumns(c ...Column) RowOpt {
	return func(r *Row) {
		r.schema = NewSchema(c...)
	}
}

// Set data to the row.
func WithRowData(a ...interface{}) RowOpt {
	return func(r *Row) {
		r.fields = a
	}
}

// Converts anything to a string. The function itself handles the common types including:
// fmt.Stringer, string, []byte, uint, int and nil. It passes anything else to the fmt.Sprintf
// to get the string representation of that value. It is used when initializing a Row instance.
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

// To cut or to enlarge input fields.
func resizeSlice(s []interface{}, become int) []interface{} {
	switch cur := len(s); {
	case cur == become:
	case cur < become:
		// Enlarged fields are set to nil.
		s = append(s, make([]interface{}, become-cur)...)
	case cur > become:
		s = s[0:become]
	}
	return s
}

// A comparator looks like this:
//   func(a, b interface{}) {
//     return a.(int) < b.(int)
//   }
// It is passed to generate a sort.Less() function.
type CmpFn func(a, b interface{}) bool

// Less() in sort.Interface
type lessFn func(i, j int) bool

// An adapter to transform Node.nodes to be sort with sort.Stable().
type sortable struct {
	nodes

	// Positions the x on this column, leaves y variant, i.e. to compare value on this field.
	col int

	count int

	// Sort in descending order
	desc bool

	less lessFn

	// A chain of func that generates a CmpFn.
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

// []*Node is a 2-dimensions slice. (table) And to sort the table's rows, we need:
//
// 1. compare in what type of the field value in which column.
// 2. all values in that column must be in identical type.
// 3. if we can sort the type of that field value.
func createSortableOn(column int, ns []*Node, opts ...SortOpt) (*sortable, error) {
	s := &sortable{
		nodes: nodes(ns),
		col:   column,
		count: len(ns),

		// Set up a fallback Less(), so that an incidental call to Less() won't panic on an unit Sortable instance.
		less: func(i, j int) bool { return true },
	}
	for _, opt := range opts {
		opt(s)
	}
	// Put the default CmpFn finder.
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
// The method executes them in order until a matcher can handle the current comparing type.
// A finder should look like this:
//   func(a interface{}) {
//     // you can do type switch on a to find a exact type of the input value,
//     // or simply ignores it if you know in advance the field type you are comparing to.
//     return func(a, b interface{}) { return a.(int) < b.(int) }
//   }
// See MatchCmp() to learn how to write a matcher.
func WithCmpMatchers(m ...func(interface{}) CmpFn) SortOpt {
	return func(s *sortable) {
		s.chain = append(s.chain, m...)
	}
}

// The default CmpFn matcher used in createSortableOn(). It uses type switch to find the type it can compare.
// It currently supports only types of string, int or time.Time.
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

// Algorithm for printing.
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
		// Means no columns to print, will panic fmt.Printf if r.FmtArgs() isn't nil
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

// Printing options are:
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

// Convenient helper to run a Printing instance.
func Print(n *Node, opts ...PrintingOpt) {
	NewPrinting(opts...).RunNode(n)
}

type PrintingOpt func(*Printing)

// Set column separator (field separator). Defaults to " ".
func WithColSep(sep string) PrintingOpt {
	return func(p *Printing) {
		p.colSep = sep
	}
}

// Set line break. Defaults to "\n".
func WithLineBrk(brk string) PrintingOpt {
	return func(p *Printing) {
		p.lineBrk = brk
	}
}

// Set writer. Defaults to os.Stdout.
func WithWriter(w io.Writer) PrintingOpt {
	return func(p *Printing) {
		p.writer = w
	}
}
