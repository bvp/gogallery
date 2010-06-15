package main

import (
	"log"
	"os"
	"sort"
	"path"
	"fmt"
	"flag"
	"http"
	"regexp"
	"template"
	
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

const maxDirDepth = 24
const templDir = "tmpl/"

var (
	fileServer = http.FileServer(".", "")
	db *sqlite.Conn

	// command flags 
    snapsize   = flag.Int("snapsize", 0, "height of the thumbnail pictures")
	picsdir = flag.String("picsdir", "pics/", "Root dir for all the pics (defaults to ./pics/)")
    dbfile   = flag.String("dbfile", "gallery.db", "File to store the db (defaults to ./gallery.db)")
    initdb    = flag.Bool("init", false, "clean out the db file and start from scratch")
)

type lines []string

func (p *lines) Write(line string) (n int, err os.Error) {
	slice := *p
    l := len(slice)
    if l == cap(slice) {  // reallocate
        // Allocate one more line
        newSlice := make([]string, l+1, l+1)
        // The copy function is predeclared and works for any slice type.
        copy(newSlice, slice)
        slice = newSlice
    }
	l++;
    slice = slice[0:l]
	slice[l-1] = line
    *p = slice
	return len(line), nil
}

type page struct {
	title	string
	body	lines
}

// http 

const lenTagPrefix = len("/tag/")
var titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")
var templates = make(map[string]*template.Template)

func renderTemplate(c *http.Conn, tmpl string, p *page) {
	err := templates[tmpl].Execute(p, c)
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
	}
}

func tagPage(tag string) (*page, os.Error) {
	stmt, err := db.Prepare(
		"SELECT file FROM tags where tag = '" + tag + "'")
	errchk(err)
	
	var t string
	var body lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&t))
		body.Write(t)
	}
	title := tag
	
	stmt.Finalize()

	return &page{title: title, body: body}, nil
}

func tagHandler(c *http.Conn, r *http.Request, urlpath string) {
	tag := urlpath[lenTagPrefix:]
	if !titleValidator.MatchString(tag) {
		http.NotFound(c, r)
		return
	}
	p, err := tagPage(tag)
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	renderTemplate(c, "tag", p)
}

func picHandler(c *http.Conn, r *http.Request, urlpath string) {
	err := r.ParseForm()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	for k, v := range (*r).Form {  
    	print(k + "\n");
    	for _, vv := range v {
    		print("	" + vv + "\n")
    	}
	}	
	var p page
	p.title = urlpath[len("/pic/"):]
	p.body = nil
	renderTemplate(c, "pic", &p)
}

func serveFile(c *http.Conn, r *http.Request) {
	fileServer.ServeHTTP(c, r);
}

func makeHandler(fn func(*http.Conn, *http.Request, string)) http.HandlerFunc {
	return func(c *http.Conn, r *http.Request) {
		title := r.URL.Path
		fn(c, r, title)
	}
}

// sqlite 
func initDb() {
	var err os.Error
	db, err = sqlite.Open(*dbfile)
	errchk(err)
	db.Exec("DROP TABLE tags")
	errchk(db.Exec("CREATE TABLE tags (file text, tag text)"))
	currentDir, err := os.Open(*picsdir, os.O_RDONLY, 0644)
	if err != nil {
		os.Exit(1)
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		os.Exit(1)
	}
	currentDir.Close()		
	sort.SortStrings(names)
	for _,v := range names {
		path := "'" + path.Join(*picsdir, v) + "'"
		errchk(db.Exec("INSERT INTO tags VALUES (" + path + ", 'all')"))
	}
	db.Close();
}

func errchk(err os.Error) {
	if err != nil {
		log.Exit(err)
	}
}

func init() {
	for _, tmpl := range []string{"tag", "pic"} {
		templates[tmpl] = template.MustParseFile(templDir + tmpl+".html", nil)
	}
	initDb()
}

func usage() {
	fmt.Fprintf(os.Stderr,
		"usage: gogallery \n");
	flag.PrintDefaults();
	os.Exit(2);
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *initdb {
		initDb()
	}
	var err os.Error
	db, err = sqlite.Open(*dbfile)
	errchk(err)

	http.HandleFunc("/tag/", makeHandler(tagHandler))
	http.HandleFunc("/pic/", makeHandler(picHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(":8080", nil)
}
