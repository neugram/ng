package parser

type Expr interface {
}

type BinaryExpr struct {
	Op    Token // Add, Sub, Mul, Div, Rem, Pow, And, Or, Equal, NotEqual, Less, Greater
	Left  Expr
	Right Expr
}

type BasicLiteral struct {
	Value interface{} // string, *big.Int, *big.Float
}

type Ident struct {
	Name string
}
