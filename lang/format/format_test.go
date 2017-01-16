package format_test

import (
	"testing"

	"neugram.io/lang/expr"
	"neugram.io/lang/format"
	"neugram.io/lang/stmt"
	"neugram.io/parser"
)

var roundTripTests = []string{
	"$$ sleep 1 && X=V Y=U env | grep X= & echo first || false; echo last $$",
	"$$ (echo a && echo b); echo c $$",
	`$$
echo one
echo two
$$`,
}

func TestRoundTrip(t *testing.T) {
	for _, src := range roundTripTests {
		s, err := parser.ParseStmt([]byte("(" + src + ")"))
		if err != nil {
			t.Errorf("ParseStmt(%q): error: %v", src, err)
			continue
		}
		if s == nil {
			t.Errorf("ParseStmt(%q): nil stmt", src)
			continue
		}
		e := s.(*stmt.Simple).Expr.(*expr.Unary).Expr
		got := format.Expr(e)
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
