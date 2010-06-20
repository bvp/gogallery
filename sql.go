package main

import (
	"os"
	"fmt"
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

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
	errchk(db.Exec("create table tags (id int, file text, tag text)"))
	errchk(scanDir(*picsdir, "all"))
}

func insert(id int, path string, tag string) {
	errchk(db.Exec(
		"insert into tags values (" +
		fmt.Sprint(id) + ", '" + path + "', '" + tag + "')"))
	maxId = id;
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

func setCurrentId(path string) {
	stmt, err := db.Prepare(
		"select id from tags where file = '" + path + "'")
	errchk(err)
	errchk(stmt.Exec())
	if stmt.Next() {
		errchk(stmt.Scan(&currentId))
	}
	stmt.Finalize()
}

func setMaxId() {
	// if we ever start to delete entries, max() won't work anymore.
	// then use a count.
	stmt, err := db.Prepare("select max(id) from tags")
	errchk(err)
	errchk(stmt.Exec())
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
