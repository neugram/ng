package format_test

import (
	"testing"

	"neugram.io/ng/format"
	"neugram.io/ng/parser"
	"neugram.io/ng/syntax/stmt"
)

var roundTripExprs = []string{
	"$$ sleep 1 && X=V Y=U env | grep X= & echo first || false; echo last $$",
	"$$ (echo a && echo b); echo c $$",
	`$$
echo one
echo two
$$`,

	// TODO: spacing around return statement
	"func(x int, y bool) ([]byte, error) {return nil, nil}",
	"func() error {return nil}",
	"func() (err error) {return nil}",
	"func(x int, y bool) (b []byte, err error) {return nil, nil}",

	"x[:y]",
	"x[y:z:t]",
	"new(int)",
}

var roundTripStmts = []string{
	"import ()",
	`import stdpath "path"`,
	`import (
	stdpath "path"
	"path/filepath"
)`,
	"type Ints []int",

	`methodik foo struct {
	S string
} {}`,

	`methodik foo struct {
	I int
} {
	func (f) F() int {return f.I}
}`,
}

func TestRoundTrip(t *testing.T) {
	var srcs []string
	for _, src := range roundTripExprs {
		srcs = append(srcs, "("+src+")")
	}
	srcs = append(srcs, roundTripStmts...)

	for _, src := range srcs {
		s, err := parser.ParseStmt([]byte(src))
		if err != nil {
			t.Errorf("ParseStmt(%q): error: %v", src, err)
			continue
		}
		if s == nil {
			t.Errorf("ParseStmt(%q): nil stmt", src)
			continue
		}
		got := format.Stmt(s)
		if got != src {
			t.Errorf("bad ouput: Expr(%q)=%q", src, got)
		}
	}
}

var typeTests = []string{
	`string`,
	`uintptr`,
	`[]interface{}`,
	`map[int64]map[string]int`,
	`struct {
	Field0     int
	Field1More <-chan struct{}
	Field2     []byte
	Filed3     struct {
		Inner1     int
		Inner2More interface{}
	}
}`,
	`func()`,
	`***int`,
	`func(func(int) bool, func() (int, error)) func() (bool, error)`,
	`interface {
	M0(int, int) (int, int)
	M1(struct{})
	M2(*int) error
}`,
	`struct{}`,
}

func TestTypes(t *testing.T) {
	for _, src := range typeTests {
		s, err := parser.ParseStmt([]byte("type x " + src))
		if err != nil {
			t.Errorf("ParseStmt(%q): error: %v", src, err)
			continue
		}
		if s == nil {
			t.Errorf("ParseStmt(%q): nil stmt", src)
			continue
		}
		typ := s.(*stmt.TypeDecl).Type
		got := format.Type(typ)
		if got != src {
			t.Errorf("bad ouput: Type(%q)=%q", src, got)
		}
	}
}
