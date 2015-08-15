// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package sqlframe

import (
	"database/sql"
	"io/ioutil"
	"os"
	"testing"

	"numgrad.io/frame"
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

func createDB(t *testing.T) (db *sql.DB, cleanup func()) {
	dbfile, err := ioutil.TempFile("", "sqlframe-sqlite-")
	if err != nil {
		t.Fatal(err)
	}
	dbfile.Close()
	os.Remove(dbfile.Name())

	db, err = sql.Open("sqlite3", dbfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		db.Close()
		os.Remove(dbfile.Name())
	}
}

func testLoadAndGet(t *testing.T, db *sql.DB) {
	f, err := Load(db, "Presidents")
	if err != nil {
		t.Fatal(err)
	}
	h, err := frame.Len(f)
	if err != nil {
		t.Fatal(err)
	}
	if h != 0 {
		t.Errorf("want zero height, got %d", h)
	}

	if _, err := frame.Copy(f, memPresidents); err != nil {
		t.Fatal(err)
	}

	numPres, err := frame.Len(memPresidents)
	if err != nil {
		t.Fatal(err)
	}

	h, err = frame.Len(f)
	if err != nil {
		t.Fatal(err)
	}
	if h != numPres {
		t.Errorf("f.Len() = %d, want %d", h, numPres)
	}

	var term2 int
	if err := f.Get(3, 0, &term2); err != nil {
		t.Errorf("Get(3, 0) error: %v", err)
	}
	if term2 != 1792 {
		t.Errorf("Get(3, 0) Washington second term %d, want 1792", term2)
	}
	var name string
	if err := f.Get(1, 0, &name); err != nil {
		t.Errorf("Get(1, 0) error: %v", err)
	}
	if want := "George Washington"; name != want {
		t.Errorf("Get(1, 0) = %q, want %q", name, want)
	}

	getTests := []struct {
		y    int
		name string
	}{
		{0, "George Washington"},
		{1, "John Adams"},
		{15, "Abraham Lincoln"},
		{3, "James Madison"},
		{0, "George Washington"},
		{17, "Ulysses S. Grant"},
	}
	for _, test := range getTests {
		var id, term1, term2 int
		var name string
		if err := f.Get(0, test.y, &id, &name, &term1, &term2); err != nil {
			t.Errorf("Get(0, %d) error: %v", test.y, err)
			continue
		}
		if test.name != name {
			t.Errorf("Get(0, %d): %q, want %q", test.y, name, test.name)
		}
	}

	// Test padding on cached path.
	if err := f.Get(3, 2, &term2); err != nil {
		t.Errorf("Get(3, 1) error: %v", err)
	}
	if term2 != 1804 {
		t.Errorf("Get(3, 1) Jefferson second term %d, want 1804", term2)
	}
}

func TestLoadAndGet(t *testing.T) {
	db, cleanup := createDB(t)
	defer cleanup()

	txt := `
	create table Presidents (
		ID integer not null primary key,
		Name text,
		Term1 int,
		Term2 int
	);`
	if _, err := db.Exec(txt); err != nil {
		t.Fatal(err)
	}

	testLoadAndGet(t, db)
}

func TestLoadAndGetNoPK(t *testing.T) {
	db, cleanup := createDB(t)
	defer cleanup()

	txt := `
	create table Presidents (
		ID integer,
		Name text,
		Term1 int,
		Term2 int
	);`
	if _, err := db.Exec(txt); err != nil {
		t.Fatal(err)
	}

	testLoadAndGet(t, db)
}
