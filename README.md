# Numengrad is a data processing package.

Numengrad is a numerical processing library written in Go. Because in
practice numerical processing is all about finding an moving data
around, and often doing parts of the processing elsewhere (for example,
inside an SQL query's WHERE clause), the library includes a small
interpreted programming language. The langauge is starting as a subset
of Go with a couple of key numerical features added.

The key data structure is the data frame. A frame has a small (known)
number of named columns and many rows.

Very imporant: it's easy to import Go packages and call Go from inside
Numengrad. Serious work should be implemented in Go, Numengrad is the
numerical glue.

# Files and Packages

Files end in *.ng*.

A package is a single file.

A file is a sequence of statements.

The first statement in a file is "package *nameoffile.ng*"

A package can be imported with "import path/to/nameoffile.ng".
In a file, imports must come directly after package. The interpreter
allows mid-file imports.

A program starts with "package main".

We borrow GOPATH.

# Types

- Allow the definition of unused Go types, but no arithmetic on unsigned ints.
- Provide an abstract type like interface{}, called val.

### Concrete types:

```
	string        (is val)
	integer       (is val)
	int64         (is val)
	float         (is val)
	float64       (is val)
	float32       (is val)
	complex128    (is val)
	[|]val        (is val)
	[|]integer    (is val, [|]val)
	[|]int64      (is val, [|]val)
	[|]float      (is val, [|]val)
	[|]float64    (is val, [|]val)
	[|]float32    (is val, [|]val)
	[|]complex128 (is val, [|]val)
```

In a [|]val, each column has an associated scalar type, which means
it's possible to extract a column from a [|]val and successfully cast
it dynamically to some numeric type that the entire table wouldn't
have casted to.

### A little generic: num

Functions and types can be parameterized over a single type parameter.
The type parameter can only resolve to numeric scalar types and must
have the name **num**. Its appearence anywhere in the type declaration
means the function is generic:

```
	min := func(x, y num) num {
		if x < y {
			return x
		}
		return y
	}
	print(min(float64(4.5), float64(4.1))) // prints 4.1
	print(min(integer(4), integer(3)))     // prints 3
	print(min(integer(4), float64(3.2)))   // compile error
```

When declaring a variable using type inference from a constant,
the variable adopts the type of num.

```
	add2 := func(x num) num {
		two := 2
		return x + two
	}
	add2(float32(4.5)) // two was a float32
	add2(int64(4))     // two was an int64
```

If in a scope where num is not assigned a type, the default numeric
type is float64.

The type [|]num can be used to represent a matrix whose type matches
the type parameter.

```
	func fill(mat [|]num, val num) {
		w, h := len(mat)
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				mat[y|x] = val
			}
		}
	}
```

### Frames

**TODO: rename to tables?**

- Dynamically, a frame keeps a type on each column, so:
	func conv(f [|]num) [|]float64 { return f.([|]float64) }
  is possible, but may fail dynamically.

- A concrete [|]float64 and the parameterized [|]num are matrixes.
  These admit arithmetic operators.

- No dimensions in the frame type parameters, that way lies too many types.

- No implicit conversions in arithmetic, even the safe ones (int64->integer) as
  they have surprising performance implications and while they may be
  possible with type parametrs, they are painful to reason about.

## Background

Start with Go's types.
- remove channels
- remove embedding
- remove ability to define interface types (TODO: revisit)
- add frame
- TODO: keep slices, or make everything use frames?
- integer (*big.Int), float (*big.Float) TODO: what precision is float?
- remove unsigned types (and arithmetic)
- keep byte, mostly a placeholder for passing to Go code
- later: introduce imaginary numbers

# Syntax

Almost entirely derived from Go.

New keywords: val, num (reserved). These don't have to be keywords,
but I'm sick of the gotchas that arise from letting people write
```type int int16```.

TODO: literal syntax for frames is giving me a headache. For now, using the composite literal syntax for the fake struct record:

```
type frame struct {
	Names []string
	Rows [][]interface{}
}
```

Must do better, but I really hate syntax.

## Frames

The Go type frame.Frame has an innocently named optional method Slice
that gets a ton of fun syntax:

Given x:
```
 Col0 Col1 Col2
0 0.0  0.1  0.2
1 1.0  1.1  1.2
2 2.0  2.1  2.2
```
or
```
x = frame{
	Names: {"Col0", "Col1", "Col2"},
	Rows: {
		{0.0, 0.1, 0.2},
		{1.0, 1.1, 1.2},
		{2.0, 2.1  2.2},
	},
}

x[1] == x["Col1"] == frame{
	Names: {"Col1"},
	Rows: {
		{0.1},
		{1.1},
		{2.1},
	},
}

x[,2] == frame{
	Names: {"Col0", "Col1", "Col2"},
	Rows: {
		{2.0, 2.1  2.2},
	},
}

x[0|2] == x["Col0"|"Col2"] == frame{
	Names: {"Col0", "Col2"},
	Rows: {
		{0.0, 0.2},
		{1.0, 1.2},
		{2.0, 2.2},
	},
}

x[0:1] == x[0|1] == x["Col0"|"Col1"]

x[1,0:1] == frame{
	Names: {"Col1"},
	Rows: {
		{0.1},
		{1.1},
	},
}

x[1,1] == 1.1 // all slicing variants return a frame, except this one
```
