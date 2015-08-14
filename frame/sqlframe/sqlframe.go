// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"

	"numgrad.io/frame"
	"numgrad.io/parser"
)

func Load(db *sql.DB, table string) (*Frame, error) {
	// TODO: if sqlite. find out by lookiing at db.Driver()?
	return sqliteLoad(db, table)
}

func NewFromFrame(db *sql.DB, table string, src frame.Frame) (*Frame, error) {
	f := &Frame{
		DB:      db,
		Table:   table,
		ColName: frame.ColumnNames(src),
	}
	if _, err := db.Exec(f.createStmt()); err != nil {
		return nil, err
	}
	return f, nil
}

type Frame struct {
	DB      *sql.DB
	Table   string
	ColName []string
	ColExpr []parser.Expr
	Where   []parser.Expr
	GroupBy []string
	Offset  int
	Limit   int

	cache interface{} // TODO
}

func (f *Frame) Get(x, y int) (interface{}, error) {
	panic("TODO")
}
func (f *Frame) Size() (width, height int) {
	return 0, 0
}
func (f *Frame) ColumnNames() []string { return f.ColName }

func (f *Frame) Accumulate(g frame.Grouping) (Frame, error) {
	panic("TODO")
}

func (f *Frame) validate() {
	// TODO: check names match a strict format, mostly to avoid SQL injection
}

func (f *Frame) createStmt() string {
	f.validate()
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "CREATE TABLE %s (\n", f.Table)
	for _, name := range f.ColName {
		fmt.Fprintf(buf, "\t%s TODO_type,\n", name)
	}
	fmt.Fprintf(buf, ");")
	return buf.String()
}

func (f *Frame) genQuery() string {
	query := "SELECT" + strings.Join(f.ColName, ", ") + " FROM " + f.Table

	if len(f.ColExpr) > 0 {
		panic("TODO ColExpr")
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
