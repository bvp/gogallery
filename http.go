package main

import (
	"os"
	"path"
	"http"
	"rand"
	"regexp"
	"template"
)

var (
	fileServer = http.FileServer(rootdir, "")
	titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")
	picValidator = regexp.MustCompile(".*(jpg|JPG|jpeg|JPEG|png|gif|GIF)$")
	refValidator = regexp.MustCompile("http://" + 
	*host + picpattern + ".*$")
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
	title	string
	host	string
	body	lines
	pic string
	tags string
	tag string
}

func renderTemplate(c *http.Conn, tmpl string, p *page) {
	err := templates[tmpl].Execute(p, c)
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
	}
}

func tagPage(tag string) *page {
	title := tag
	pics := getPics(tag)
	for i := 0; i<len(pics); i++ {
		dir, file := path.Split(pics[i])
		thumb := path.Join(dir, thumbsDir, file)
		pics[i] = "<a href=\"http://" + *host + picpattern +
			pics[i] + "\"><img src=\"http://" + *host + "/" +
			thumb + "\"/></a>"
	}
	return &page{title: title, host: *host, body: pics, pic: picpattern, tags: tagspattern, tag: tagpattern}
}

func tagsPage() *page {
	title := "All tags"
	tags := getTags()
	return &page{title: title, host: *host, body: tags, pic: picpattern, tags: tagspattern, tag: tagpattern}
}

func tagHandler(c *http.Conn, r *http.Request, urlpath string) {
	tag := urlpath[len(tagpattern):]
	if !titleValidator.MatchString(tag) {
		http.NotFound(c, r)
		return
	}
	p := tagPage(tag)
	renderTemplate(c, "tag", p)
}

func picHandler(c *http.Conn, r *http.Request, urlpath string) {
	var p page
	p.title = urlpath[len(picpattern):]
	p.body = nil
	p.host = *host
	p.pic = picpattern
	p.tags = tagspattern
	p.tag = tagpattern
	err := r.ParseForm()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	setCurrentId(p.title)
	// get new tag from POST
	for k, v := range (*r).Form {
		if k == "newtag" {
			// only allow single alphanumeric word tag 
			if titleValidator.MatchString(v[0]) {
				insert(maxId+1, p.title, v[0])
			}
			break
    	}
	}	
	renderTemplate(c, "pic", &p)
}

func tagsHandler(c *http.Conn, r *http.Request, urlpath string) {
	p := tagsPage()
	renderTemplate(c, "tags", p)
}

func randomHandler(c *http.Conn, r *http.Request, urlpath string) {
	randId := rand.Intn(maxId) + 1
	s := selectById(randId)
	s = picpattern + s
	http.Redirect(c, s, http.StatusFound)
}

func nextHandler(c *http.Conn, r *http.Request, urlpath string) {
	if !refValidator.MatchString((*r).Referer) {
		http.NotFound(c, r)
		return
	}
	currentId++;
	if currentId > maxId {
		currentId = 1
	}
	s := selectById(currentId)
	s = picpattern + s
	http.Redirect(c, s, http.StatusFound)
}

func prevHandler(c *http.Conn, r *http.Request, urlpath string) {
	if !refValidator.MatchString((*r).Referer) {
		http.NotFound(c, r)
		return
	}
	currentId--;
	if currentId < 1 {
		currentId = maxId
	}
	s := selectById(currentId)
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
