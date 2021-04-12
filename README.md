
# Overview

Pprint is a library to typeset output with auto-width padding. It's implemented using a tree and has directory-like context. Supports typesetting or sorting per directory context.

* Auto column width calculation
* Fixed width, left alignment, no padding
* Sort on raw value
* Folder context: different typesets or sorting per folder

# Concepts

Pprint is built on a tree structure.

Each tree node maintains its own "folder layout", which is represented by ColumnSchema, for its child ndoes.

A ColumnSchema can be shared among entire tree.

Each node hold a Row contains input data and with a reference to an instance of ColumnSchema.

# Installing

    go get -u github.com/adios/pprint
    
Next, include pprint in your application:

```go
import "github.com/adios/pprint"
```

# Examples

```
package main

import (
	"fmt"
	"time"
	
	pp "github.com/adios/pprint"
)

func main() {
	var (
		pt = func(date string) time.Time {
			t, _ := time.Parse("2006-01-02", date)
			return t
		}
		data = [][]interface{}{
			{21196, "Keep On Truckin'", pt("1999-05-17"), "ahote glowtusks", 9.75},
			{-1162, "Cry Wolf", pt("2007-10-16"), "adahy windshot", 4.22},
			{-1248, "Needle In a Haystack", pt("1988-09-06"), "shikpa longmoon", 0.7},
			{50994, "Greased Lightning", pt("1989-06-04"), "helushka emberhair", 2.72},
			{80640, "Let Her Rip", pt("1981-01-13"), "geashkoo grassdream", 1.6},
			{50997, "Up In Arms", pt("1981-01-13"), "oonnak hardrage", 0.58},
		}
	)

	n := pp.NewNode()
	for _, row := range data {
		_, err := n.Push(row...)
		if err != nil {
			panic(err)
		}
	}
	// All push are auto-width by default
	pp.Print(n)

	fmt.Println("===")

	// Or using a customized a typeset
	n = pp.NewNode(
		pp.WithColumns(
			pp.NewColumn(),
			pp.NewColumn(pp.WithLeftAlignment()),
			pp.NewColumn(),
			pp.NewColumn(pp.WithWidth(24)),
			// Width 0 means no padding
			pp.NewColumn(pp.WithWidth(0)),
		),
	)
	for _, row := range data {
		n.Push(row...)
	}
	pp.Print(n, pp.WithColSep("|"))

	fmt.Println("===")

	// Use with a folder-like context
	n = pp.NewNode()
	// Data 0, 1, 2 are under node n
	n.Push(data[0]...)
	m, _ := n.Push(data[1]...)
	n.Push(data[2]...)

	// Data 3, 4, 5 are under node m, which is the position of data 1
	m.Push(data[3]...)
	m.Push(data[4]...)
	m.Push(data[5]...)

	// Output order would be 0, 1, 3, 4, 5, 2
	pp.Print(n)
}

```

Output:

```
Output:
21196     Keep On Truckin' 1999-05-17 00:00:00 +0000 UTC     ahote glowtusks 9.75
-1162             Cry Wolf 2007-10-16 00:00:00 +0000 UTC      adahy windshot 4.22
-1248 Needle In a Haystack 1988-09-06 00:00:00 +0000 UTC     shikpa longmoon  0.7
50994    Greased Lightning 1989-06-04 00:00:00 +0000 UTC  helushka emberhair 2.72
80640          Let Her Rip 1981-01-13 00:00:00 +0000 UTC geashkoo grassdream  1.6
50997           Up In Arms 1981-01-13 00:00:00 +0000 UTC     oonnak hardrage 0.58
===
21196|Keep On Truckin'    |1999-05-17 00:00:00 +0000 UTC|         ahote glowtusks|9.75
-1162|Cry Wolf            |2007-10-16 00:00:00 +0000 UTC|          adahy windshot|4.22
-1248|Needle In a Haystack|1988-09-06 00:00:00 +0000 UTC|         shikpa longmoon|0.7
50994|Greased Lightning   |1989-06-04 00:00:00 +0000 UTC|      helushka emberhair|2.72
80640|Let Her Rip         |1981-01-13 00:00:00 +0000 UTC|     geashkoo grassdream|1.6
50997|Up In Arms          |1981-01-13 00:00:00 +0000 UTC|         oonnak hardrage|0.58
===
21196     Keep On Truckin' 1999-05-17 00:00:00 +0000 UTC     ahote glowtusks 9.75
-1162             Cry Wolf 2007-10-16 00:00:00 +0000 UTC      adahy windshot 4.22
50994    Greased Lightning 1989-06-04 00:00:00 +0000 UTC  helushka emberhair 2.72
80640          Let Her Rip 1981-01-13 00:00:00 +0000 UTC geashkoo grassdream  1.6
50997           Up In Arms 1981-01-13 00:00:00 +0000 UTC     oonnak hardrage 0.58
-1248 Needle In a Haystack 1988-09-06 00:00:00 +0000 UTC     shikpa longmoon  0.7
```
