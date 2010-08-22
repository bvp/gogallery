package main

import (
	"os"
	"path"
)

const basicTemplates = ".tmpl"
const picTmpl= "pic.html"
const tagTmpl= "tag.html"
const tagsTmpl= "tags.html"
const uploadTmpl= "upload.html"

//TODO: replace the hardcoded template names with the above const everywhere if possible
var (
pic_html = `
<div>
<center>
<table>
<tr>
<td>
<a href="http://{host}/prev"> prev </a>
</td>
<td>
<a href="http://{host}{tags}"> tags </a>
</td>
<td>
<a href="http://{host}/next"> next </a>
</td>
</table>
</center>
</div>

<div>
<center>
<a href="http://{host}/random">
<img src="http://{host}/{title}" alt={title} />
</a>
</center>
</div>

<div> 
<center>
<form action="{pic}{title}" method="post"> 
<input type="text" name="newtag"/> 
<input type="submit" value="Tag!"> 
</center>
</form> 
</div> 
`

tag_html = `
<h1><center>{title}</center></h1>

<div>
<center>
<a href="http://{host}{tags}"> tags </a>
</center>
</div>

<div>
<center>
{.repeated section body}
{@}
{.end}
</center>
</div>
`

tags_html = `
<h1><center>{title}</center></h1>

<div>
<center>
{.repeated section body}
<a href="http://{host}{tag}{@}">{@}</a>
{.end}
</center>
</div>
`
upload_html = `
<div> 
<center>
<form action="{upload}" enctype="multipart/form-data" method="post">
Upload <input type="file" name="upload" size="40"> <br>
Tag <input type="text" name="tag" size="30"> <br>
<input name="submit" value="submit" type="submit"><input type="reset">
</center>
</form> 
</div>
 
<div> 
<center>
<p>
{title}
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
	
	tmpls := [][2]string{[2]string{pic_html, picTmpl}, [2]string{tag_html, tagTmpl}, [2]string{tags_html, tagsTmpl}, [2]string{upload_html, uploadTmpl}}
	for _, tmpl := range tmpls {
		templ, err := os.Open(path.Join(dirpath, tmpl[1]), os.O_CREAT|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, err = templ.WriteString(tmpl[0]);
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
