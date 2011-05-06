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

//TODO: replace the hardcoded template names with the above const everywhere if possible
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
<a href="http://{Host}{Tags}"> tags </a>
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
<img src="http://{Host}/{Title}" alt="{Title}" />
</a>
</center>
</div>

<div> 
<center>
<form action="{Pic}{Title}" method="post"> 
<input type="text" name="newtag"/> 
<input type="submit" value="Tag!"> 
</center>
</form> 
</div> 
`

	tag_html = `
<h1><center>{Title}</center></h1>

<div>
<center>
<a href="http://{Host}{Tags}"> tags </a>
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
<a href="http://{Host}{Tag}{@}">{@}</a>
{.end}
</center>
</div>
`
	upload_html = `
<div> 
<center>
<form action="{Upload}" enctype="multipart/form-data" method="post">
Upload <input type="file" name="upload" size="40"> <br>
Tag <input type="text" name="tag" size="30"> <br>
<input name="submit" value="submit" type="submit"><input type="reset">
</center>
</form> 
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
