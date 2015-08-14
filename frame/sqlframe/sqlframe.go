// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
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
		ColName: append([]string{}, src.Cols()...),
	}
	if _, err := db.Exec(f.createStmt()); err != nil {
		return nil, err
	}
	return f, nil
}

type Frame struct {
	DB      *sql.DB
	Table   string
	ColName []string // TODO unexport ColName?
	// TODO ColType
	ColExpr []parser.Expr
	Where   []parser.Expr
	GroupBy []string
	Offset  int
	Limit   int

	insert *sql.Stmt
	count  *sql.Stmt
	cache  interface{} // TODO
}

func (f *Frame) Get(x, y int, dst ...interface{}) error {
	panic("TODO")
}

func (f *Frame) Height() (int, error) {
	if f.count == nil {
		var err error
		f.count, err = f.DB.Prepare("SELECT COUNT(*) FROM " + f.Table + ";")
		if err != nil {
			return 0, fmt.Errorf("sqlframe: %v", err)
		}
	}
	rows, err := f.count.Query()
	if err != nil {
		return 0, err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("sqlframe: %v", err)
		}
		return 0, fmt.Errorf("sqlframe: table %q returned no count", f.Table)
	}
	count := 0
	if err := rows.Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlframe: %v", err)
	}
	return count, rows.Close()
}

func (f *Frame) CopyFrom(src frame.Frame) (n int, err error) {
	if f.insert == nil {
		buf := new(bytes.Buffer)
		fmt.Fprintf(buf, "INSERT INTO %s (", f.Table)
		fmt.Fprintf(buf, strings.Join(f.ColName, ", "))
		fmt.Fprintf(buf, ") VALUES (")
		for i := range f.ColName {
			if i > 0 {
				fmt.Fprintf(buf, ", ")
			}
			fmt.Fprintf(buf, "?")
		}
		fmt.Fprintf(buf, ");")
		var err error
		f.insert, err = f.DB.Prepare(buf.String())
		if err != nil {
			return 0, fmt.Errorf("sqlframe: %v", err)
		}
	}

	// TODO: fast path for src.(*Frame): insert from select

	row := make([]interface{}, len(f.ColName))
	rowp := make([]interface{}, len(row))
	for i := range row {
		rowp[i] = &row[i]
	}
	y := 0
	for {
		err := src.Get(0, y, rowp...)
		if err == io.EOF {
			break // last row, all is good
		}
		if err != nil {
			return y, err
		}
		if _, err := f.insert.Exec(row...); err != nil {
			return y, fmt.Errorf("sqlframe: %v", err)
		}
		y++
	}
	return y, nil
}

func (f *Frame) Cols() []string { return f.ColName }

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
