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
)

/*
TODO composition of filters

Slice, Filter, and Accumulate interact oddly.

Given a filter, f.Filter("term1 < 1808"), we may get the query
	select name, term1 from presidents where term1 < 1808;
which gives us
	{1, "George Washington", 1789, 1792},
	{2, "John Adams", 1797, 0},
	{3, "Thomas Jefferson", 1800, 1804},
this could be sliced: f.Filter("term1 < 1808").Slice(0, 2, 0, 2) into
	{1, "George Washington", 1789, 1792},
	{2, "John Adams", 1797, 0},
by adding to the query:
	select name, term1 from presidents where term1 < 1808 limit 2;
so far so good.

However, if we first applied an offset slice, then the filter cannot
simply be added. That is,
	f.Slice(0, 2, 2, 5).Filter("term1 < 1808")
needs to produce
	{3, "Thomas Jefferson", 1800, 1804},
which is the query:
	select name, term1 from (
		select name, term1 from presidents offset 2 limit 5;
	) where term1 < 1808;
.

So we need to introduce a new kind of subFrame that can correctly
compose these restrictions. Or at the very least realize when they
don't compose, and punt to the default impl.
*/

// TODO: Set always returns an error on an accumulation

func Load(db *sql.DB, table string) (frame.Frame, error) {
	// TODO: if sqlite. find out by lookiing at db.Driver()?
	return sqliteLoad(db, table)
}

func NewFromFrame(db *sql.DB, table string, src frame.Frame) (frame.Frame, error) {
	f := &sqlFrame{
		db:        db,
		table:     table,
		sliceCols: append([]string{}, src.Cols()...),
	}
	if _, err := db.Exec(f.createStmt()); err != nil {
		return nil, err
	}
	return f, nil
}

type sqlFrame struct {
	db         *sql.DB
	table      string
	sliceCols  []string // table columns that are part of the frame
	primaryKey []string // primary key columns

	// TODO colExpr    []parser.Expr
	// TODO where      []parser.Expr
	// TODO groupBy    []string
	offset int
	limit  int // -1 for no limit

	// TODO colType

	insert   *sql.Stmt
	count    *sql.Stmt
	rowForPK *sql.Stmt

	cache struct {
		rowPKs [][]interface{} // rowPKs[i], primary key for row i
		curGet *sql.Rows       // current forward cursor, call Next for row len(rowPKs)
	}
}

func (f *sqlFrame) Get(x, y int, dst ...interface{}) (err error) {
	//fmt.Printf("Get(%d, %d): len(f.cache.rowPKs)=%d\n", x, y, len(f.cache.rowPKs))
	var empty interface{}
	if x > 0 {
		dst = append(make([]interface{}, x), dst...)
		for i := 0; i < x; i++ {
			dst[i] = &empty
		}
	}
	if w := len(dst); w < len(f.sliceCols) {
		dst = append(dst, make([]interface{}, len(f.sliceCols)-len(dst))...)
		for i := w; i < len(dst); i++ {
			dst[i] = &empty
		}
	}

	if y < len(f.cache.rowPKs) {
		// Previously visited row.
		// Extract it from the DB using the primary key.
		if f.rowForPK == nil {
			buf := new(bytes.Buffer)
			fmt.Fprint(buf, "SELECT ")
			fmt.Fprint(buf, strings.Join(f.sliceCols, ", "))
			fmt.Fprintf(buf, " FROM %s WHERE ", f.table)
			for i, key := range f.primaryKey {
				if i > 0 {
					fmt.Fprintf(buf, " AND ")
				}
				fmt.Fprintf(buf, "%s=?", key)
			}
			fmt.Fprintf(buf, ";")
			f.rowForPK, err = f.db.Prepare(buf.String())
			if err != nil {
				return fmt.Errorf("sqlframe: %v", err)
			}
		}
		row := f.rowForPK.QueryRow(f.cache.rowPKs[y]...)
		return row.Scan(dst...)
	}
	if f.cache.curGet == nil {
		f.cache.rowPKs = nil
		f.cache.curGet, err = f.db.Query(f.queryForGet())
		if err != nil {
			return fmt.Errorf("sqlframe: %v", err)
		}
	}
	for y >= len(f.cache.rowPKs) {
		if !f.cache.curGet.Next() {
			f.cache.curGet = nil
			return io.EOF
		}
		pk := make([]interface{}, len(f.primaryKey))
		pkp := make([]interface{}, len(f.primaryKey))
		for i := range pk {
			pkp[i] = &pk[i]
		}
		err = f.cache.curGet.Scan(append(dst, pkp...)...)
		if err != nil {
			f.cache.curGet = nil
			return fmt.Errorf("sqlframe: %v", err)
		}
		f.cache.rowPKs = append(f.cache.rowPKs, pk)
	}
	return nil
}

func (f *sqlFrame) Len() (int, error) {
	if f.count == nil {
		var err error
		f.count, err = f.db.Prepare("SELECT COUNT(*) FROM " + f.table + ";")
		if err != nil {
			return 0, fmt.Errorf("sqlframe: %v", err)
		}
	}
	row := f.count.QueryRow()
	count := 0
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlframe: count %v", err)
	}
	count -= f.offset
	if f.limit >= 0 && count > f.limit {
		count = f.limit
	}
	return count, nil
}

func (f *sqlFrame) CopyFrom(src frame.Frame) (n int, err error) {
	if f.insert == nil {
		buf := new(bytes.Buffer)
		fmt.Fprintf(buf, "INSERT INTO %s (", f.table)
		fmt.Fprintf(buf, strings.Join(f.sliceCols, ", "))
		fmt.Fprintf(buf, ") VALUES (")
		for i := range f.sliceCols {
			if i > 0 {
				fmt.Fprintf(buf, ", ")
			}
			fmt.Fprintf(buf, "?")
		}
		fmt.Fprintf(buf, ");")
		var err error
		f.insert, err = f.db.Prepare(buf.String())
		if err != nil {
			return 0, fmt.Errorf("sqlframe: %v", err)
		}
	}

	// TODO: fast path for src.(*sqlFrame): insert from select

	row := make([]interface{}, len(f.sliceCols))
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

func (f *sqlFrame) Cols() []string { return f.sliceCols }

func (d *sqlFrame) Slice(x, xlen, y, ylen int) frame.Frame {
	n := &sqlFrame{
		db:         d.db,
		table:      d.table,
		sliceCols:  d.sliceCols[x : x+xlen],
		primaryKey: d.primaryKey,
		count:      d.count,
		offset:     d.offset + y,
		limit:      ylen,
	}
	if len(d.cache.rowPKs) > y {
		n.cache.rowPKs = d.cache.rowPKs[y:]
		if len(n.cache.rowPKs) > ylen {
			n.cache.rowPKs = n.cache.rowPKs[:ylen]
		}
	}
	return n
}

func (f *sqlFrame) Accumulate(g frame.Grouping) (frame.Frame, error) {
	panic("TODO")
}

func (f *sqlFrame) validate() {
	// TODO: check names match a strict format, mostly to avoid SQL injection
}

func (f *sqlFrame) createStmt() string {
	f.validate()
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "CREATE TABLE %s (\n", f.table)
	for _, name := range f.sliceCols {
		fmt.Fprintf(buf, "\t%s TODO_type,\n", name)
	}
	fmt.Fprintf(buf, ");")
	return buf.String()
}

func (f *sqlFrame) queryForGet() string {
	f.validate()
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "SELECT ")
	col := 0
	for _, c := range f.sliceCols {
		if col > 0 {
			fmt.Fprintf(buf, ", ")
		}
		col++
		fmt.Fprintf(buf, c)
	}
	for i, c := range f.primaryKey {
		if col > 0 {
			fmt.Fprintf(buf, ", ")
		}
		col++
		fmt.Fprintf(buf, "%s as _pk%d", c, i)
	}
	fmt.Fprintf(buf, " FROM %s", f.table)
	if f.limit >= 0 {
		fmt.Fprintf(buf, " LIMIT %d", f.limit)
	}
	if f.offset > 0 {
		fmt.Fprintf(buf, " OFFSET %d", f.offset)
	}
	fmt.Fprintf(buf, ";")
	// TODO where
	// TODO groupBy
	// TODO offset
	// TODO limit
	// TODO colExpr
	return buf.String()
}
