package sqlframe

import (
	"database/sql"
	"strings"

	"numgrad.io/frame"
	"numgrad.io/parser"
)

type Frame struct {
	DB         *sql.DB
	Table      string
	Column     []string
	ColumnExpr []parser.Expr
	Where      []parser.Expr
	GroupBy    []string
	Offset     int
	Limit      int

	cache interface{} // TODO
}

func (f *Frame) ColumnName(x int) string { return f.Column[x] }

func (f *Frame) Accumulate(g frame.Grouping) (Frame, error) {
	panic("TODO")
}

func (f *Frame) genQuery() string {
	query := "SELECT" + strings.Join(f.Column, ", ") + " FROM " + f.Table

	if len(f.ColumnExpr) > 0 {
		panic("TODO ColumnExpr")
	}
	if len(f.Where) > 0 {
		panic("TODO Where")
	}
	if len(f.GroupBy) > 0 {
		query += " GROUP BY " + strings.Join(f.GroupBy, ", ")
	}

	query += ";"
	return query
}
