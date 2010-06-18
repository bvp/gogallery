package main

import (
	"log"
	"os"
	"syscall"
	"sort"
	"path"
	"fmt"
	"flag"
	"http"
	"regexp"
	"template"
	"rand"
//	"strings"
	
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

const maxDirDepth = 24
const templDir = "tmpl/"
const thumbsDir = ".thumbs"
const picpattern = "/pic/"
const tagpattern = "/tag/"
const tagspattern = "/tags"
const randompattern = "/random"

var (
	rootDir, _ = os.Getwd();
	fileServer = http.FileServer(rootDir, "")
	rootDirLen = len(rootDir)
	db *sqlite.Conn
	titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")
	templates = make(map[string]*template.Template)
	picValidator = regexp.MustCompile(".*(jpg|JPG|jpeg|JPEG|png)$")
	maxId = 1

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
	pics	lines
	thumbs	lines
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
		"select file from tags where tag = '" + tag + "'")
	errchk(err)
	
	var t string
	var pics lines
	var thumbs lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&t))
		pics.Write(t)
		dir, file := path.Split(t)
		thumb := path.Join(dir, thumbsDir, file)
		thumbs.Write(thumb)
	}
	title := tag
	
	stmt.Finalize()

	return &page{title: title, pics: pics, thumbs: thumbs}, nil
}

//TODO: tags size
func tagsPage() (*page, os.Error) {
	stmt, err := db.Prepare(
		"select tag, count(tag) from tags group by tag")
	errchk(err)
	
	var s string
	var i int
	var pics lines
	errchk(stmt.Exec())
	for stmt.Next() {
		errchk(stmt.Scan(&s, &i))
		pics.Write(s)
	}
	title := "All tags"
	
	stmt.Finalize()

	return &page{title: title, pics: pics, thumbs: nil}, nil
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
	p.pics = nil
	p.thumbs = nil
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
				"insert into tags values (" + 
					fmt.Sprint(maxId) + ", '" + p.title + "', '" + newtag + "')"))
				maxId++;
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

//TODO: set maxId when initDb has not been run
func randomHandler(c *http.Conn, r *http.Request, urlpath string) {
	randId := rand.Intn(maxId) + 1
	stmt, err := db.Prepare(
		"select file from tags where id = " + fmt.Sprint(randId))
	errchk(err)

	var s string
	errchk(stmt.Exec())
	if stmt.Next() {
		errchk(stmt.Scan(&s))
	}
	stmt.Finalize()
	s = picpattern + s
	http.Redirect(c, s, http.StatusFound)
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
	db.Exec("drop table tags")
	errchk(db.Exec("create table tags (id int, file text, tag text)"))
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
	e := 0
	e = syscall.Mkdir(path.Join(dirpath, thumbsDir), 0755) 
	if e != 0 && e != syscall.EEXIST {
		return os.Errno(e)
	}
	for _,v := range names {
		childpath := path.Join(dirpath, v)
		fi, err := os.Lstat(childpath)
		if err != nil {
			return err
		}
		if fi.IsDirectory() && v != thumbsDir {
			err = scanDir(childpath)
			if err != nil {
				return err
			}
		} else {
			if picValidator.MatchString(childpath) {
				err = mkThumb(childpath)
				if err != nil {
					return err
				}
				path := childpath[rootDirLen+1:]
				errchk(db.Exec(
					"insert into tags values (" +
						fmt.Sprint(maxId) + ", '" + path + "', 'all')"))
				maxId++;
			}
		}

	}
	return err
}

func mkThumb(filepath string) os.Error {
	dir, file := path.Split(filepath)
	thumb := path.Join(dir, thumbsDir, file)
	fd, err := os.Open(thumb, os.O_CREAT|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		if err != os.EEXIST {
			return err
		}
		return nil
	}
	fd.Close()
	var args []string = make([]string, 5)
	args[0] = "/usr/bin/convert"
	args[1] = filepath
	args[2] = "-thumbnail"
	args[3] = "200x300"
	args[4] = thumb
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	pid, err := os.ForkExec(args[0], args, os.Environ(), "", fds)
	if err != nil {
		return err
	}
	_, err = os.Wait(pid, os.WNOHANG)
	if err != nil {
		return err
	}
	return nil
}

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
	http.HandleFunc(randompattern, makeHandler(randomHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(":8080", nil)
}
