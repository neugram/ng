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

## Current thoughts on types

- Allow the definition of unused Go types, but no arithmetic on unsigned ints.

- Concrete types:
	string        (is val)
	integer       (is val, ari)
	int64         (is val, ari)
	float         (is val, ari)
	float64       (is val, ari)
	float32       (is val, ari)
	complex128    (is val, ari)
	[|]integer    (is val, ari, [|]val, [|]num)
	[|]int64      (is val, ari, [|]val, [|]num)
	[|]float      (is val, ari, [|]val, [|]num)
	[|]float64    (is val, ari, [|]val, [|]num)
	[|]float32    (is val, ari, [|]val, [|]num)
	[|]complex128 (is val, ari, [|]val, [|]num)

- Abstract types:
	- val: everything is a val. A.K.A. interface{}.
	- num: basic numeric types: integer, float, int64, etc
	- ari: nums and frames of nums. Admits arithmetic, 
	- [|]val: each cell has any type. Can dynamic cast to other frames.
	- [|]num: frame of basic numeric types

- All possible frame types:
	[|]num
	[|]integer
	[|]int64
	[|]float
	[|]float64
	[|]float32
	[|]complex128

- Introduce type parameters.
  Parameters must be restricted to being either num, ari, [|]num.

	type <T num> struct { x T }
	type <T [|]num> struct { x T }
	func <T num> min(x, y T) T {
		if x < y {
			return x
		}
		return y
	}
	func <T num, U [|]num> fill(mat U, val T) {
		for y := 0; y < mat.Height; y++ {
			for x := 0; x < mat.Width(); x++ {
				mat[y|x] = val
			}
		}
	}

- Dynamically, a frame keeps a type on each column, so:
	func conv(f [|]num) [|]float64 { return f.([|]float64) }
  is possible, but may fail dynamically.

- A [|]float64 (etc) is a matrix. These admit arithmetic operators.

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
- limit numeric types:
	int64
	integer (*big.Int)
	float32
	float64
	float (*big.Float)
	TODO: what precision is float?
- remove unsigned types (and arithmetic)
- keep byte, mostly a placeholder for passing to Go code
- later: introduce imaginary numbers

- extremely tempting, dynamic types:
	```
	func min(x, y val) val {
		if x < y {
			return x
		}
		return y
	}
	```
  Interpreter would initially box all types, a JIT could unbox and specialize
  the important types, int64/float{32,64}, and in the future possibly custom
  user types.

  IMPORTANT:
	```
	x, y := int64(4), float64(5)
	min(x, y) // fails, dynamic check in (<) operator
	```
  Avoid nasty implicit type conversions.
  Possible implicit conversions we could introduce later:
	- int64 -> int
	- float32 -> float
	- float64 -> float
  These are "safe" promotions.

- problem: frames are dynamically typed, with runtime basic types
  attached to each column. there are three potential concepts worth
  encoding in the type system:
	frame<<float64>> all fields are of type float64, dynamic cols/rows
  this is a dynamically sized matrix.
	frame<string, float64, int> three columns, types specified
	frame<float64|4:5> a 4x5 matrix
  all of these have uses, but do any of them have enough uses to justify
  The most compelling case is matricies, but that can be dealt with by
  introducing a matrix type with a type parameter. the specific-size
  matricies can be built by hand out of structs.

  The type implications of Permute and Slice become incredibly complex
  if we allow per-column types. It can probably be type checked, but
  would you want to program with them?

# Syntax

Almost entirely derived from Go.

New keywords: frame, val (reserved)

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
