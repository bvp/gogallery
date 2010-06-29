package main

import (
	"os"
	"path"
	"http"
	"rand"
	"regexp"
	"template"
	"strings"
	"io/ioutil"
	"bytes"
	"time"
//	"log"
)

const maxupload int = 2e6

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
	title	string
	host	string 
	body	lines
	pic string 
	tags string 
	tag string 
	upload string
}

type Request2 struct {
	*http.Request
	upload []byte
}

func NewRequest2(r *http.Request, upload []byte) *Request2 {
    return &Request2{r, upload}
}

// ParseForm parses the request body as a form for POST requests, or the raw query for GET requests.
// It is idempotent.
func (r *Request2) ParseForm() (err os.Error) {
    if r.Form != nil {
        return
    }

    var query string
    switch r.Method {
    case "GET":
        query = r.URL.RawQuery
    case "POST":
        if r.Body == nil {
            r.Form = make(map[string][]string)
            return os.ErrorString("missing form body")
        }
        ct := r.Header["Content-Type"]
        switch strings.Split(ct, ";", 2)[0] {
        case "text/plain", "application/x-www-form-urlencoded", "":
            var b []byte
            if b, err = ioutil.ReadAll(r.Body); err != nil {
                r.Form = make(map[string][]string)
                return err
            }
            query = string(b)
       case "multipart/form-data":
            boundary := strings.Split(ct, "boundary=", 2)[1]
            var b []byte
            if b, err = ioutil.ReadAll(r.Body); err != nil {
                return err
            }
            parts := bytes.Split(b, []byte("--"+boundary+"--\r\n"), 0)
            parts = bytes.Split(parts[0], []byte("--"+boundary+"\r\n"), 0)
            for _, data := range parts {
                if len(data) < 2 {
                    continue
                }
                data = data[0 : len(data)-2] // remove the \r\n
                var line []byte
                var rest = data
                //content-disposition params
                cdparams := map[string]string{}
                for {
                    res := bytes.Split(rest, []byte{'\r', '\n'}, 2)
                    if len(res) != 2 {
                        break
                    }
                    line = res[0]
                    rest = res[1]
                    if len(line) == 0 {
                        break
                    }

                    header := strings.Split(string(line), ":", 2)
                    n := strings.TrimSpace(header[0])
                    v := strings.TrimSpace(header[1])
                    if n == "Content-Disposition" {
                        cdparts := strings.Split(v, ";", 0)
                        for _, cdparam := range cdparts[1:] {
                            split := strings.Split(cdparam, "=", 2)
                            pname := strings.TrimSpace(split[0])
                            pval := strings.TrimSpace(split[1])
                            cdparams[pname] = pval
                        }
                    }
                }
                //if the param doesn't have a name, ignore it
                if _, ok := cdparams["name"]; !ok {
                    continue
                }
                name := cdparams["name"]
                //check if name is quoted
                if strings.HasPrefix(name, `"`) {
                    name = name[1 : len(name)-1]
                }
                // if it's a file, store it in the upload member,
				// and add the filename to the query
                if filename, ok := cdparams["filename"]; ok {
                    if strings.HasPrefix(filename, `"`) {
                        filename = filename[1 : len(filename)-1]
                    }
					if len(rest) > maxupload {
						err = os.NewError("upload too large")
						return err
					}
					copy(r.upload, rest)
					r.upload = r.upload[0:len(rest)]
					query += "&upload=" + filename
					continue
                }
				// if it's the tag, add it to the query
				if name == "tag" {
					query += "&tag=" + string(rest)
				}
            }
        default:
            r.Form = make(map[string][]string)
			err = os.NewError("unknown Content-Type")
			return err
        }
    }
    r.Form, err = http.ParseQuery(query)
    return
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
	return &page{title: title, host: *host, body: pics, tags: tagspattern}
}

func tagsPage() *page {
	title := "All tags"
	tags := getTags()
	return &page{title: title, body: tags, host: *host, tag: tagpattern}
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
	p := page{title: urlpath[len(picpattern):], host: *host,
		tags: tagspattern, pic: picpattern}
	err := r.ParseForm()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	currentId = getCurrentId(p.title)
	// get new tag from POST
	for k, v := range (*r).Form {
		if k == "newtag" {
			// only allow single alphanumeric word tag 
			if titleValidator.MatchString(v[0]) {
				insert(p.title, v[0])
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
	s := selectNext(randId)
	if s == "" {
		s = selectPrev(randId)
	}
	if s == "" {
		http.NotFound(c, r)
		return
	}
	s = picpattern + s
	http.Redirect(c, s, http.StatusFound)
}

//TODO: check that referer can never have a different *host part ?
func nextHandler(c *http.Conn, r *http.Request, urlpath string) {
	ok, err := regexp.MatchString(
		"^http://"+*host+picpattern+".*$", (*r).Referer)
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
//TODO: maybe print the 1st one instead of a 404 ?
	if !ok {		
		http.NotFound(c, r)
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
	http.Redirect(c, s, http.StatusFound)
}

func prevHandler(c *http.Conn, r *http.Request, urlpath string) {
	ok, err := regexp.MatchString(
		"^http://"+*host+picpattern+".*$", (*r).Referer)
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}
	if !ok {		
		http.NotFound(c, r)
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
	http.Redirect(c, s, http.StatusFound)
}

func uploadHandler(c *http.Conn, r *http.Request, urlpath string) {
	p := page{title: ""}
	tag := ""
	filepath := ""
	upload := make([]byte, maxupload)
	r2 := NewRequest2(r, upload)
	err := r2.ParseForm()
	if err != nil {
		http.Error(c, err.String(), http.StatusInternalServerError)
		return
	}

	// if "upload" is in the form, we got a new file, so write it to disk
	for k, v := range (*r).Form {
		if k == "upload" {
			// write file in dir with YYYY-MM-DD format
			filedir := path.Join(*picsdir, time.UTC().Format("2006-01-02"))
			err = mkdir(filedir)
			if err != nil {
				http.Error(c, err.String(), http.StatusInternalServerError)
				return
			}
			// create thumbsdir while we're at it
			err = mkdir(path.Join(filedir, thumbsDir))
			if err != nil {
				http.Error(c, err.String(), http.StatusInternalServerError)
				return
			}
			// finally write the file
			filepath = path.Join(filedir, v[0])
			err := ioutil.WriteFile(filepath, r2.upload, 0644)
			if err != nil {
				http.Error(c, err.String(), http.StatusInternalServerError)
				return
			}
			p.title = v[0] + ": upload sucessfull"
			if tag != "" {
				break
			}
			continue
    	}
		if k == "tag" {
			tag = v[0]
			if p.title != "" {
				break;
			}
		}
	}

	// only insert tag if we have an upload of a pic and a tag for it			
	if tag != "" && p.title != "" {
		if titleValidator.MatchString(tag) && 
			picValidator.MatchString(filepath) {
			err = mkThumb(filepath)
			if err != nil {
				http.Error(c, err.String(), http.StatusInternalServerError)
				return 
			}
			insert(filepath[rootdirlen+1:], tag)
		}
	}

	renderTemplate(c, "upload", &p)
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
