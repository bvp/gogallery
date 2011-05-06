package main

import (
	"bytes"
	"os"
	"path"
	"http"
	"rand"
	"regexp"
	"template"
	"strings"
	"io/ioutil"
	"smtp"
	"time"
//	"fmt"
)

const (
	maxupload = 10e6
	largeupload = 2e6
	fileinform = "upload"
	taginform = "tag"
	idstring = "gogallery"
)

var (
	fileServer = http.FileServer(rootdir, "")
	titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")
	picValidator = regexp.MustCompile(".*(jpg|JPG|jpeg|JPEG|png|gif|GIF)$")
	templates = make(map[string]*template.Template)
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
	Title	string
	Host	string 
	Body	lines
	Pic string 
	Tags string 
	Tag string 
	Upload string
}

func newPage(title string, body lines) *page {
	p := page{title, *host, body, picpattern, tagspattern, tagpattern, uploadpattern}
	return &p
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *page) {
	err := templates[tmpl].Execute(w, p)
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
	}
}

func tagPage(tag string) *page {
	pics := getPics(tag)
	for i := 0; i<len(pics); i++ {
		dir, file := path.Split(pics[i])
		thumb := path.Join(dir, thumbsDir, file)
		pics[i] = "<a href=\"http://" + *host + picpattern +
			pics[i] + "\"><img src=\"http://" + *host + "/" +
			thumb + "\"/></a>"
	}
	return newPage(tag, pics)
}

func tagsPage() *page {
	title := "All tags"
	tags := getTags()
	return newPage(title, tags)
}

func tagHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	tag := urlpath[len(tagpattern):]
	if !titleValidator.MatchString(tag) {
		http.NotFound(w, r)
		return
	}
	p := tagPage(tag)
	renderTemplate(w, tagName, p)
}

func picHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	p := newPage(urlpath[len(picpattern):], nil)
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
		return
	}
	currentId = getCurrentId(p.Title)
	// get new tag from POST
	for k, v := range (*r).Form {
		if k == "newtag" {
			// only allow single alphanumeric word tag 
			if titleValidator.MatchString(v[0]) {
				insert(p.Title, v[0])
			}
			break
    	}
	}	
	renderTemplate(w, picName, p)
}

func tagsHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	p := tagsPage()
	renderTemplate(w, tagsName, p)
}

func randomHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	randId := rand.Intn(maxId) + 1
	s := selectNext(randId)
	if s == "" {
		s = selectPrev(randId)
	}
	if s == "" {
		http.NotFound(w, r)
		return
	}
	s = picpattern + s
	http.Redirect(w, r, s, http.StatusFound)
}

//TODO: check that referer can never have a different *host part ?
func nextHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	ok, err := regexp.MatchString(
		"^http://"+*host+picpattern+".*$", (*r).Referer)
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
		return
	}
//TODO: maybe print the 1st one instead of a 404 ?
	if !ok {		
		http.NotFound(w, r)
		return
	}
	prefix := len("http://" + *host + picpattern)
	file := (*r).Referer[prefix:]
	currentId = getCurrentId(file)
	s := selectNext(currentId)
	if s == "" {
		s = file
	}
	s = picpattern + s
	http.Redirect(w, r, s, http.StatusFound)
}

func prevHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	ok, err := regexp.MatchString(
		"^http://"+*host+picpattern+".*$", (*r).Referer)
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
		return
	}
	if !ok {		
		http.NotFound(w, r)
		return
	}
	prefix := len("http://" + *host + picpattern)
	file := (*r).Referer[prefix:]
	currentId = getCurrentId(file)
	s := selectPrev(currentId)
	if s == "" {
		s = file
	}
	s = picpattern + s
	http.Redirect(w, r, s, http.StatusFound)
}

func uploadHandler(w http.ResponseWriter, r *http.Request, urlpath string) {
	p := newPage("", nil)
	tag := ""
	filepath := ""

	reader, err := r.MultipartReader()

	// do nothing if no form	
	if err == nil {
		for {
			part, err := reader.NextPart()
			if err != nil {
				http.Error(w, err.String(), http.StatusInternalServerError)
				return
			}
			if part == nil {
				break
			}
			partName := part.FormName()
			// get the file
			if partName == fileinform {
				// get the filename 
//TODO: use new native func to do that
				line := part.Header.Get("Content-Disposition")
				filename := line[strings.Index(line, "filename="):]
//TODO: that will fail if it's not at the end of the line, do better with a regex
				filename = filename[10:len(filename)-1]
				// get the upload
//TODO: sizing the buffer and then checking n indeed prevents filling mem and/or disk, but the thing will still be uploaded fully -> waste of b/w. => do better?
				b := bytes.NewBuffer(make([]byte, 0, maxupload))
				n, err := b.ReadFrom(part)
				if err != nil {
					if err != os.EOF {
						http.Error(w, err.String(), http.StatusInternalServerError)
						return
					}
				}
				if n > maxupload {
					err = os.NewError("upload too large")
					http.Error(w, err.String(), http.StatusInternalServerError)
					return
				}
				if n > largeupload && *conffile != "" {
					err = smtp.SendMail(conf.Server, nil, conf.From, conf.To, []byte(conf.Message))
					if err != nil {
						http.Error(w, err.String(), http.StatusInternalServerError)
						return 
					}
				}
				// write file in dir with YYYY-MM-DD format
				filedir := path.Join(*picsdir, time.UTC().Format("2006-01-02"))
				err = mkdir(filedir)
				if err != nil {
					http.Error(w, err.String(), http.StatusInternalServerError)
					return
				}
				// create thumbsdir while we're at it
				err = mkdir(path.Join(filedir, thumbsDir))
				if err != nil {
					http.Error(w, err.String(), http.StatusInternalServerError)
					return
				}
				// finally write the file
				filepath = path.Join(filedir, filename)
				err = ioutil.WriteFile(filepath, b.Bytes(), 0644)
				if err != nil {
					http.Error(w, err.String(), http.StatusInternalServerError)
					return
				}
				p.Title = filename + ": upload sucessfull"
				if tag != "" {
					break
				}
				continue
			}
			// get the tag
			if partName == taginform {
				b := make([]byte, 128)
				n, err := part.Read(b)
//TODO: better err handling ?
				if err == nil {
					b = b[0:n]
					tag = string(b)
				}
				if p.Title != "" {
					break;
				}
			}
		}
		// only insert tag if we have an upload of a pic and a tag for it			
		if tag != "" && p.Title != "" {
			if titleValidator.MatchString(tag) && 
				picValidator.MatchString(filepath) {
				err = mkThumb(filepath)
				if err != nil {
					http.Error(w, err.String(), http.StatusInternalServerError)
					return 
				}
				insert(filepath[rootdirlen+1:], tag)
			}
		}
	}
		
	renderTemplate(w, upName, p)
}

func serveFile(w http.ResponseWriter, r *http.Request) {
	fileServer.ServeHTTP(w, r);
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		title := r.URL.Path
		w.Header().Set("Server", idstring)
		fn(w, r, title)
	}
}
