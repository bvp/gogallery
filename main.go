package main

import (
	"flag"
	"fmt"
	"http"
	"image"
	"image/png"
	"image/jpeg"
	"json"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"template"
	sqlite "gosqlite.googlecode.com/hg/sqlite"
)

//TODO: enable/disable public tags? auth?
//TODO: add last selected tag (and all tags) as a link?
//TODO: cloud of tags? variable font size?
//TODO: preload all the picsPaths for a given tag?

const (
	thumbsDir = ".thumbs"
	resizedDir = ".resized"
	picpattern = "/pic/"
	tagpattern = "/tag/"
	tagspattern = "/tags"
// security through obscurity for now; just don't advertize your uploadpattern if you don't want others to upload to your server
	uploadpattern = "/upload"
	allPics = "all"
)

var (
	rootdir, _ = os.Getwd()
	rootdirlen = len(rootdir)
	config conf = conf{
		Dbfile: "./gallery.db",
		Initdb: false,
		Picsdir: "./",
		Thumbsize: "200x300",
		Normalsize: "800x600",
		Tmpldir: "",
		Norand: false}
	//TODO: derive from a geometry
	maxWidth int
	maxHeight int
)

var (
	conffile = flag.String("conf", "", "json conf file to send email alerts")
	host       = flag.String("host", "localhost:8080", "listening port and hostname that will appear in the urls")
	help       = flag.Bool("h", false, "show this help")
)

type conf struct {
	Email emailConf
	Dbfile string
	Initdb bool
	Picsdir string
	Thumbsize string
	Normalsize string
	Tmpldir string
	Norand bool
}

type emailConf struct {
	Server string
	From string
	To []string
	Message string
}

func readConf(confFile string) os.Error {
	r, err := os.Open(confFile)
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(r)
	err = dec.Decode(&config)
	if err != nil {
		log.Fatal(err)
	}
	r.Close()
	fmt.Printf("%v \n", config)
	sizes := strings.Split(config.Normalsize, "x", -1)
	if len(sizes) != 2 { 
		return os.NewError("Invalid Normalsize value \n")
	}
	maxWidth, err = strconv.Atoi(sizes[0])
	errchk(err)
	maxHeight, err = strconv.Atoi(sizes[1])
	errchk(err)
	return nil
}

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
	currentDir, err := os.Open(dirpath)
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
	for _, v := range names {
		childpath := path.Join(dirpath, v)
		fi, err := os.Lstat(childpath)
		if err != nil {
			return err
		}
		if fi.IsDirectory() && v != thumbsDir && v != resizedDir {
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
	args[3] = config.Thumbsize
	args[4] = thumb
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, &os.ProcAttr{Files: fds})
	if err != nil {
		return err
	}
	_, err = os.Wait(p.Pid, os.WSTOPPED)
	if err != nil {
		return err
	}
	return nil
}

//TODO: mv to an image.go file
func needResize(pic string) bool {
	var err os.Error
	var im image.Image
	f, err := os.Open(pic)
	if err != nil {
		log.Fatal(err)
	}
	switch filepath.Ext(pic) {
	case ".png":
		im, err = png.Decode(f)
	case ".jpeg", ".jpg":
		im, err = jpeg.Decode(f)
	default:
		log.Print("unsupported image file: ", filepath.Ext(pic))
		return false
	}
	if err != nil {
//TODO: not fatal
		log.Fatal(err)
	}
	w := im.Bounds().Dx()
	h := im.Bounds().Dy()
	return w > maxWidth || h > maxHeight
}

// we can use convert -resize/-scale because, like -thumbnail, they conserve proportions 
func mkResized(pic string) os.Error {
	dir, file := path.Split(pic)
	resized := path.Join(dir, resizedDir, file)
	_, err := os.Stat(resized)
	if err == nil {
		return nil
	}
	err = os.MkdirAll(path.Join(dir, resizedDir), 0755)
	if err != nil {
		return err
	}	
	args := []string{"/usr/bin/convert", pic, "-resize", config.Normalsize, resized}
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, &os.ProcAttr{Files: fds})
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
	config.Picsdir = path.Clean(config.Picsdir)
	if (config.Picsdir)[0] != '/' {
		cwd, _ := os.Getwd()
		config.Picsdir = path.Join(cwd, config.Picsdir)
	}
	pathValidator := regexp.MustCompile(rootdir + ".*")
	if !pathValidator.MatchString(config.Picsdir) {
		log.Fatal("picsdir has to be a subdir of rootdir. (symlink ok)")
	}
}

func chktmpl() {
	if config.Tmpldir == "" {
		config.Tmpldir = basicTemplates
		err := mkTemplates(config.Tmpldir)
		if err != nil {
			log.Fatal(err)
		}
	}
	// same drill for templates.
	config.Tmpldir = path.Clean(config.Tmpldir)
	if (config.Tmpldir)[0] != '/' {
		cwd, _ := os.Getwd()
		config.Tmpldir = path.Join(cwd, config.Tmpldir)
	}
	pathValidator := regexp.MustCompile(rootdir + ".*")
	if !pathValidator.MatchString(config.Tmpldir) {
		log.Fatal("tmpldir has to be a subdir of rootdir. (symlink ok)")
	}
	for _, tmpl := range []string{tagName, picName, tagsName, upName} {
		templates[tmpl] = template.MustParseFile(path.Join(config.Tmpldir, tmpl+".html"), nil)
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
	fmt.Fprintf(os.Stderr, "usage: \n\t gogallery [-picsdir=\"dir\"] [-dbfile=\"file\"] tag tagname\n")
	fmt.Fprintf(os.Stderr, "usage: \n\t gogallery [-dbfile=\"file\"] deltag tagname \n")
	fmt.Fprintf(os.Stderr, "\t gogallery \n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}

	if *conffile != "" {
		errchk(readConf(*conffile))
	}
	chkpicsdir()

	// tag or del cmds 
	nargs := flag.NArg()
	if nargs > 0 {
		if nargs < 2 {
			usage()
		}
		var err os.Error
		cmd := flag.Args()[0]
		switch cmd {
		case "tag":
//why the smeg can't I use := here? 
			db, err = sqlite.Open(config.Dbfile)
			errchk(err)
			errchk(scanDir(config.Picsdir, flag.Args()[1]))
			log.Print("Scanning of " + config.Picsdir + " complete.")
			db.Close()
		case "deltag":
			db, err = sqlite.Open(config.Dbfile)
			errchk(err)
			delete(flag.Args()[1])
			db.Close()
		default:
			usage()		
		}
		return
	}

	// web server mode
	chktmpl()
	if config.Initdb {
		initDb()
	} else {
		var err os.Error
		db, err = sqlite.Open(config.Dbfile)
		errchk(err)
	}
	setMaxId()

	http.HandleFunc(tagpattern, makeHandler(tagHandler))
	http.HandleFunc(picpattern, makeHandler(picHandler))
	http.HandleFunc(tagspattern, makeHandler(tagsHandler))
	http.HandleFunc("/random", makeHandler(randomHandler))
	http.HandleFunc("/next", makeHandler(nextHandler))
	http.HandleFunc("/prev", makeHandler(prevHandler))
	http.HandleFunc(uploadpattern, makeHandler(uploadHandler))
	http.HandleFunc("/", http.HandlerFunc(serveFile))
	http.ListenAndServe(*host, nil)
}
