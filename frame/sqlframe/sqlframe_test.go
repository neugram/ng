// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"database/sql"
	"testing"

	"numgrad.io/frame/memframe"

	_ "github.com/mattn/go-sqlite3"
)

var memPresidents = memframe.NewLiteral(
	[]string{"ID", "Name", "Term1", "Term2"},
	[][]interface{}{
		{1, "George Washington", 1789, 1792},
		{2, "John Adams", 1797, 0},
		{3, "Thomas Jefferson", 1800, 1804},
		{4, "James Madison", 1808, 1812},
		{5, "James Monroe", 1816, 1820},
		{6, "John Quincy Adams", 1824, 0},
		{7, "Andrew Jackson", 1828, 1832},
		{8, "Martin Van Buren", 1836, 0},
		{9, "William Henry Harrison", 1840, 0},
		{10, "John Tyler", 1840, 0},
		{11, "James K. Polk", 1844, 0},
		{12, "Zachary Taylor", 1848, 0},
		{13, "Millard Fillmore", 1848, 0},
		{14, "Franklin Pierce", 1852, 0},
		{15, "James Buchanan", 1856, 0},
		{16, "Abraham Lincoln", 1860, 1864},
		{17, "Andrew Johnson", 1864, 0},
		{18, "Ulysses S. Grant", 1868, 1872},
	},
)

func TestSQLBasics(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	txt := `
	create table presidents (
		ID integer not null primary key,
		Name text,
		Term1 int,
		Term2 int
	);`
	if _, err = db.Exec(txt); err != nil {
		t.Fatal(err)
	}

	f, err := Load(db, "presidents")
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := f.Size(); w != 4 {
		t.Errorf("want width 4, got %d", w)
	}
}
