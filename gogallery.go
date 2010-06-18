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
//	"strings"
	
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

const maxDirDepth = 24
const templDir = "tmpl/"
const picpattern = "/pic/"
const tagpattern = "/tag/"
const tagspattern = "/tags"
const thumbsdir = ".thumbs"

var (
	rootDir, _ = os.Getwd();
	fileServer = http.FileServer(rootDir, "")
	rootDirLen = len(rootDir)
	db *sqlite.Conn
	titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")
	templates = make(map[string]*template.Template)
	picValidator = regexp.MustCompile(".*(jpg|JPG|jpeg|JPEG|png)$")

	// command flags 
    snapsize   = flag.Int("snapsize", 0, "height of the thumbnail pictures")
	picsdir = flag.String("picsdir", "pics/", "Root dir for all the pics (defaults to ./pics/)")
    dbfile   = flag.String("dbfile", "./gallery.db", "File to store the db (defaults to ./gallery.db)")
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

//TODO: tags size
func tagsPage() (*page, os.Error) {
	stmt, err := db.Prepare(
		"select tag, count(tag) from tags group by tag")
	errchk(err)
	
	var s string
	var i int
	var body lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&s, &i))
		body.Write(s)
	}
	title := "All tags"
	
	stmt.Finalize()

	return &page{title: title, body: body}, nil
}

func tagHandler(c *http.Conn, r *http.Request, urlpath string) {
	tag := urlpath[len(tagpattern):]
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
	var p page
	p.title = urlpath[len(picpattern):]
	p.body = nil
	err := r.ParseForm()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	// get newtag from POST
	for k, v := range (*r).Form {
		if k == "newtag" {
			newtag := v[0]
			// only allow single alphanumeric word tag 
			if titleValidator.MatchString(newtag) {
				errchk(db.Exec(
					"INSERT INTO tags VALUES ('" + p.title + "', '" + newtag + "')"))
			}
			break
    	}
	}	
	renderTemplate(c, "pic", &p)
}

func tagsHandler(c *http.Conn, r *http.Request, urlpath string) {
	p, err := tagsPage()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	renderTemplate(c, "tags", p)
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
//	err = scanDir(*picsdir)
	err = scanDir(*picsdir)
	if err != nil {
		log.Exit(err)
	}

	db.Close();
}

func errchk(err os.Error) {
	if err != nil {
		log.Exit(err)
	}
}

func scanDir(dirpath string) os.Error {
	currentDir, err := os.Open(dirpath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		return err
	}
	currentDir.Close()		
	sort.SortStrings(names)
	for _,v := range names {
		childpath := path.Join(dirpath, v)
		fi, err := os.Lstat(childpath)
		if err != nil {
			return err
		}
		if fi.IsDirectory() {
			scanDir(childpath)
		} else {
			if picValidator.MatchString(childpath) {
//				mkThumb(childpath)
				path := childpath[rootDirLen+1:]
				errchk(db.Exec(
					"INSERT INTO tags VALUES ('" + path + "', 'all')"))
			}
		}

	}
	return err
}

/*
func mkThumb(filepath string) {
	dir, file := path.Split(filepath)
	thumb := path.Join(dir, thumbsdir, file)
	//TODO: don't do it if file exists
	var args2 []string = make([]string, 2)
	args[0] = path.Join("/usr/bin/convert")
	args[1] = fullpath
	args[2] = "-thumbnail"
	args[3] = "200x300"
	args[4] = "200x300"
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	os.ForkExec(args[0], args, os.Environ(), "", fds)
}
*/

func init() {
	for _, tmpl := range []string{"tag", "pic", "tags"} {
		templates[tmpl] = template.MustParseFile(templDir + tmpl+".html", nil)
	}
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
	
	*picsdir = path.Clean(*picsdir)
	if (*picsdir)[0] != '/' {
		cwd, _ := os.Getwd() 
		*picsdir = path.Join(cwd, *picsdir)
	}
	
	if *initdb {
		initDb()
	}
	var err os.Error
	db, err = sqlite.Open(*dbfile)
	errchk(err)

	http.HandleFunc(tagpattern, makeHandler(tagHandler))
	http.HandleFunc(picpattern, makeHandler(picHandler))
	http.HandleFunc(tagspattern, makeHandler(tagsHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(":8080", nil)
}
