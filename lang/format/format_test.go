package format

import (
	"testing"

	"neugram.io/lang/expr"
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
		got := Expr(e)
		if got != src {
			t.Errorf("bad ouput: Expr(%q)=%q", src, got)
		}
	}
}
