// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"database/sql"
	"fmt"
	"math/big"

	"neugram.io/frame"
)

func sqliteLoad(db *sql.DB, table string) (frame.Frame, error) {
	// TODO validate table name
	f := &sqlFrame{
		db:    db,
		table: table,
		limit: -1,
	}
	rows, err := db.Query("pragma table_info('" + table + "');")
	if err != nil {
		return nil, fmt.Errorf("sqlframe.Load: %v", err)
	}
	pkComponents := make(map[int]string)
	defer rows.Close()
	for rows.Next() {
		var num int
		var name string
		var ty string
		var empty interface{}
		var pk int
		if err := rows.Scan(&num, &name, &ty, &empty, &empty, &pk); err != nil {
			return nil, fmt.Errorf("sqlframe.Load: %v", err)
		}
		f.sliceCols = append(f.sliceCols, name)
		if pk > 0 {
			pkComponents[pk-1] = name
		}
	}
	if len(pkComponents) == 0 {
		// An SQLite table without a primary key has a hidden primary
		// key column called rowid. Add it to the list of all columns
		// (but explicitly not the slice columns) and use it.
		f.primaryKey = []string{"rowid"}
	} else {
		f.primaryKey = make([]string, len(pkComponents))
		for pos, name := range pkComponents {
			f.primaryKey[pos] = name
		}
	}

	return f, nil
}

func sqliteScanBegin(frameDst []interface{}) ([]interface{}, error) {
	sqlDst := make([]interface{}, len(frameDst))
	for i, d := range frameDst {
		switch d.(type) {
		case *int64, *float32, *float64, *string:
			sqlDst[i] = d
		case *big.Int:
			sqlDst[i] = new(int64)
		case *big.Float:
			sqlDst[i] = new(float64)
		default:
			return nil, fmt.Errorf("sqlframe: unsupported Get parameter type %T", d)
		}
	}
	return sqlDst, nil
}

func sqliteScanEnd(frameDst, sqlDst []interface{}) {
	for i, d := range frameDst {
		switch d := d.(type) {
		case *int64, *float32, *float64, *string:
			// do nothing
		case *big.Int:
			d.SetInt64(*sqlDst[i].(*int64))
		case *big.Float:
			d.SetFloat64(*sqlDst[i].(*float64))
		default:
			panic(fmt.Sprintf("sqlframe: unsupported Get parameter type %T", d))
		}
	}
}
