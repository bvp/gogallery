package main

import (
	"os"
	"path"
	"regexp"
)

const basicTemplates = ".tmpl"
const tagName = "tag"
const picName = "pic"
const tagsName = "tags"
const upName = "upload"

var (
	pic_html = `
<div>
<center>
<table>
<tr>
<td>
<a href="http://{Host}/prev"> prev </a>
</td>
<td>
<a href="http://{Host}` + tagspattern + `"> tags </a>
</td>
<td>
<a href="http://{Host}/random"> rand </a>
</td>
<td>
<a href="http://{Host}/next"> next </a>
</td>
</table>
</center>
</div>

<div>
<center>
<a href="http://{Host}/random">
<img src="http://{Host}/{.repeated section Body}{@}{.end}" alt="{Title}" />
</a>
</center>
</div>

<div> 
<center>
<form action="` + picpattern + `{Title}" method="post"> 
<input type="text" name="` + newtag + `"/> 
<input type="submit" value="Tag!"> 
</form>
<form action="` + picpattern + `{Title}" method="post"> 
<input type="hidden" name="` + fullsize + `"/> 
<input type="submit" value="Full size">
</form>
</center>
</div> 
`

	tag_html = `
<h1><center>{Title}</center></h1>

<div>
<center>
<a href="http://{Host}` + tagspattern + `"> tags </a>
</center>
</div>

<div>
<center>
{.repeated section Body}
{@}
{.end}
</center>
</div>
`

	tags_html   = `
<h1><center>{Title}</center></h1>

<div>
<center>
{.repeated section Body}
<a href="http://{Host}` + tagpattern + `{@}">{@}</a>
{.end}
</center>
</div>
`
//TODO: more suitable name for input submit below? probably no?
	upload_html = `

<div> 
<center>
<p>
Upload and optionally tag a file
</p>
</center>
</div>

<div> 
<center>
<form action=` + uploadpattern + ` enctype="multipart/form-data" method="post">
Upload <input type="file" name="upload" size="40"> <br>
Tag <input type="text" name="tag" size="30"> <br>
<input type="submit" value="Upload" >
</form> 
</center>
</div>

<div> 
<center>
<p>
{Title}
</p>
</center>
</div>

`
)

func mkTemplates(dirpath string) os.Error {
	err := mkdir(dirpath)
	if err != nil {
		return err
	}

	if *norand {
		randHtml := regexp.MustCompile(`<a href="http://{Host}/random">
`)
		pic_html = randHtml.ReplaceAllString(pic_html,
			`<a href="http://{Host}/{Title}">
`)
	}

	tmpls := [][2]string{[2]string{pic_html, picName + ".html"}, [2]string{tag_html, tagName + ".html"}, [2]string{tags_html, tagsName + ".html"}, [2]string{upload_html, upName + ".html"}}
	for _, tmpl := range tmpls {
		templ, err := os.Create(path.Join(dirpath, tmpl[1]))
		if err != nil {
			return err
		}
		_, err = templ.WriteString(tmpl[0])
		if err != nil {
			return err
		}
		err = templ.Close()
		if err != nil {
			return err
		}
	}
	return err
}
