package main

import (
	"os"
	"fmt"
	"log"
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

//TODO: sql constraints on the id

var (
	db *sqlite.Conn
	maxId = 0
	currentId = maxId
)

func initDb() {
	var err os.Error
	db, err = sqlite.Open(*dbfile)
	errchk(err)
	db.Exec("drop table tags")
	errchk(db.Exec(
		"create table tags (id integer primary key, file text, tag text)"))
	errchk(scanDir(*picsdir, "all"))
	log.Stdout("Scanning of " + *picsdir + " complete.")
}

//TODO: if insert stmt returns the id, use that to set maxId
func insert(path string, tag string) {
	errchk(db.Exec(
		"insert into tags values (NULL, '" +
		path + "', '" + tag + "')"))
	maxId++;
}

func selectById(id int) string {
	stmt, err := db.Prepare(
		"select file from tags where id = " + fmt.Sprint(id))
	errchk(err)

	var s string
	errchk(stmt.Exec())
	if stmt.Next() {
		errchk(stmt.Scan(&s))
	}
	stmt.Finalize()
	return s
}

func getCurrentId(path string) int {
	stmt, err := db.Prepare(
		"select id from tags where file = '" + path + "'")
	errchk(err)
	errchk(stmt.Exec())
	var i int
	if stmt.Next() {
		errchk(stmt.Scan(&i))
	}
	stmt.Finalize()
	return i
}

func setMaxId() {
	// if we ever start to delete entries, max() won't work anymore.
	// then use a count.
	// check db sanity
	stmt, err := db.Prepare("select count(id) from tags")
	errchk(err)
	errchk(stmt.Exec())
	var i int
	if stmt.Next() {
		errchk(stmt.Scan(&i))
	}
	stmt.Finalize()
	if i == 0 {
		log.Exit("empty db. fill it with with -init or -tagmode")
	}
	// now do the real work
	stmt, err = db.Prepare("select max(id) from tags")
	errchk(err)
	errchk(stmt.Exec())
//BUG: Next() returns true when select max(id)... results in an empty set
	if stmt.Next() {
		errchk(stmt.Scan(&maxId))
	}
	stmt.Finalize()
}

//TODO: use the count to set the tags sizes
func getTags() lines {
	stmt, err := db.Prepare(
		"select tag, count(tag) from tags group by tag")
	errchk(err)
	
	var s string
	var i int
	var tags lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&s, &i))
		tags.Write(s)
	}
	stmt.Finalize()
	return tags
}

func getPics(tag string) lines {
	stmt, err := db.Prepare(
		"select file from tags where tag = '" + tag + "'")
	errchk(err)
	
	var s string
	var pics lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&s))
		pics.Write(s)
	}
	stmt.Finalize()
	return pics
}
