// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"database/sql"
	"fmt"
)

func sqliteLoad(db *sql.DB, table string) (*Frame, error) {
	// TODO validate table name
	f := &Frame{
		DB:    db,
		Table: table,
	}
	rows, err := db.Query("pragma table_info('" + table + "');")
	if err != nil {
		return nil, fmt.Errorf("sqlframe.Load: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var num int
		var name string
		var ty string
		var empty interface{}
		if err := rows.Scan(&num, &name, &ty, &empty, &empty, &empty); err != nil {
			return nil, fmt.Errorf("sqlframe.Load: %v", err)
		}
		fmt.Printf("%d: %s\n", num, name)
	}

	return f, fmt.Errorf("sqlframe.Load: TODO")
}
