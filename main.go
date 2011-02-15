package main

import (
	"os"
	"syscall"
	"sort"
	"path"
	"fmt"
	"flag"
	"http"
	"template"
	"log"
	"regexp"
	"strings"
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

const thumbsDir = ".thumbs"
const picpattern = "/pic/"
const tagpattern = "/tag/"
const tagspattern = "/tags"
// security through obscurity for now; just don't advertize your uploadpattern if you don't want others to upload to your server
const uploadpattern = "/upload"

//TODO: allow _ and - in tagnames
var (
	rootdir, _ = os.Getwd();
	rootdirlen = len(rootdir)
	// command flags 
	dbfile   = flag.String("dbfile", "./gallery.db", "File to store the db")
	host = flag.String("host", "localhost:8080", "hostname and port for this server that will appear in the urls")
	hostlisten = flag.String("hostlisten", "", "hostname and port on which this server really listen (defaults to -host value)")
	initdb    = flag.Bool("init", false, "clean out the db file and start from scratch")
	picsdir = flag.String("picsdir", "./", "Root dir for all the pics")
	thumbsize   = flag.String("thumbsize", "200x300", "size of the thumbnails")
	tmpldir = flag.String("tmpldir", "", "dir for the templates. generates basic ones in " + basicTemplates + " by default")
	norand = flag.Bool("norand", false, "disable random for when clicking on image")
	help = flag.Bool("h", false, "show this help")
)

func mkdir(dirpath string) os.Error {
	// used syscall because can't figure out how to check EEXIST with os
	e := 0
	e = syscall.Mkdir(dirpath, 0755) 
	if e != 0 && e != syscall.EEXIST {
		return os.Errno(e)
	}
	return nil
}

func scanDir(dirpath string, tag string) os.Error {
	currentDir, err := os.Open(dirpath, os.O_RDONLY, 0744)
	if err != nil {
		return err
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		return err
	}
	currentDir.Close()		
	sort.SortStrings(names)
	err = mkdir(path.Join(dirpath, thumbsDir)) 
	if err != nil {
		return err
	}
	for _,v := range names {
		childpath := path.Join(dirpath, v)
		fi, err := os.Lstat(childpath)
		if err != nil {
			return err
		}
		if fi.IsDirectory() && v != thumbsDir {
			err = scanDir(childpath, tag)
			if err != nil {
				return err
			}
		} else {
			if picValidator.MatchString(childpath) {
				err = mkThumb(childpath)
				if err != nil {
					return err
				}
				path := childpath[rootdirlen+1:]
				insert(path, tag)
			}
		}

	}
	return err
}

//TODO: set up a pool of goroutines to do the converts concurrently (probably not a win on a monocore though)
func mkThumb(filepath string) os.Error {
	dir, file := path.Split(filepath)
	thumb := path.Join(dir, thumbsDir, file)
	_, err := os.Stat(thumb)
	if err == nil {
		return nil
	}
	var args []string = make([]string, 5)
	args[0] = "/usr/bin/convert"
	args[1] = filepath
	args[2] = "-thumbnail"
	args[3] = *thumbsize
	args[4] = thumb
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, os.Environ(), "", fds)
	if err != nil {
		return err
	}
	_, err = os.Wait(p.Pid, os.WSTOPPED)
	if err != nil {
		return err
	}
	return nil
}

func chkpicsdir() {
	// fullpath for picsdir. must be within document root
	*picsdir = path.Clean(*picsdir)
	if (*picsdir)[0] != '/' {
		cwd, _ := os.Getwd() 
		*picsdir = path.Join(cwd, *picsdir)
	}
	pathValidator := regexp.MustCompile(rootdir + ".*")
	if !pathValidator.MatchString(*picsdir) {
		log.Fatal("picsdir has to be a subdir of rootdir. (symlink ok)")
	}
}

func chktmpl() {
	if *tmpldir == "" {
		*tmpldir = basicTemplates
		err := mkTemplates(*tmpldir)
		if err != nil {
			log.Fatal(err)
		}
	}
	// same drill for templates.
	*tmpldir = path.Clean(*tmpldir)
	if (*tmpldir)[0] != '/' {
		cwd, _ := os.Getwd() 
		*tmpldir = path.Join(cwd, *tmpldir)
	}
	pathValidator := regexp.MustCompile(rootdir + ".*")
	if !pathValidator.MatchString(*tmpldir) {
		log.Fatal("tmpldir has to be a subdir of rootdir. (symlink ok)")
	}
	for _, tmpl := range []string{"tag", "pic", "tags", "upload"} {
		templates[tmpl] = template.MustParseFile(path.Join(*tmpldir, tmpl+".html"), nil)
	}
}

//TODO: add some other potential risky chars
func badchar(filepath string) (bool, string) {
	len0 := len(filepath)
	n := strings.IndexRune(filepath, '\'')
	for n != -1 {
		filepath = filepath[0:n] + filepath[n+1:]
		n = strings.IndexRune(filepath, '\'')
	}
	if len(filepath) != len0 {
		return true, filepath
	}
	return false, ""
}

func errchk(err os.Error) {
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: \n\t gogallery [-picsdir=\"dir\"] tag \n");
	fmt.Fprintf(os.Stderr, "\t gogallery \n");
	flag.PrintDefaults();
	os.Exit(2);
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}
	
	chkpicsdir()

	// tagging mode
	if flag.NArg() > 0 {
		tag := flag.Args()[0]
		var err os.Error
		db, err = sqlite.Open(*dbfile)
		errchk(err)
		errchk(scanDir(*picsdir, tag))
		log.Print("Scanning of " + *picsdir + " complete.")
		db.Close()
		return
	}

	// web server mode
	chktmpl()
	if *initdb {
		initDb()
	} else {
		var err os.Error
		db, err = sqlite.Open(*dbfile)
		errchk(err)
	}
	setMaxId()
	if len(*hostlisten) == 0 {
		*hostlisten = *host
	}
	http.HandleFunc(tagpattern, makeHandler(tagHandler))
	http.HandleFunc(picpattern, makeHandler(picHandler))
	http.HandleFunc(tagspattern, makeHandler(tagsHandler))
	http.HandleFunc("/random", makeHandler(randomHandler))
	http.HandleFunc("/next", makeHandler(nextHandler))
	http.HandleFunc("/prev", makeHandler(prevHandler))
	http.HandleFunc(uploadpattern, makeHandler(uploadHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(*hostlisten, nil)
}
