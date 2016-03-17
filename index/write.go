// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"html/template"
	"net/http"

	"github.com/coreos/mantle/Godeps/_workspace/src/google.golang.org/api/storage/v1"
)

var (
	indexTemplate *template.Template
)

const (
	INDEX_TEXT = `<html>
    <head>
	<title>{{.Bucket}}/{{.Prefix}}</title>
	<meta http-equiv="X-Clacks-Overhead" content="GNU Terry Pratchett" />
    </head>
    <body>
    <h1>{{.Bucket}}/{{.Prefix}}</h1>
    {{range $name, $sub := .SubDirs}}
	[dir] <a href="{{$name}}">{{$name}}</a> </br>
    {{end}}
    {{range $name, $obj := .Objects}}
	{{if ne $name "index.html"}}
	    [file] <a href="{{$name}}">{{$name}}</a> </br>
	{{end}}
    {{end}}
    </body>
</html>
`
)

func init() {
	indexTemplate = template.Must(template.New("index").Parse(INDEX_TEXT))
}

// crcSum returns the base64 encoded CRC32c sum of the given data
func crcSum(b []byte) string {
	c := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	c.Write(b)
	return base64.StdEncoding.EncodeToString(c.Sum(nil))
}

// Judges whether two Objects are equal based on size and CRC
func crcEq(a, b *storage.Object) bool {
	return a.Size == b.Size && a.Crc32c == b.Crc32c
}

func (d *Directory) UpdateIndex(client *http.Client) error {
	service, err := storage.New(client)
	if err != nil {
		return err
	}

	if len(d.SubDirs) == 0 && len(d.Objects) == 0 {
		return nil
	}

	buf := bytes.Buffer{}
	err = indexTemplate.Execute(&buf, d)
	if err != nil {
		return err
	}

	obj := &storage.Object{
		Name:         d.Prefix + "index.html",
		ContentType:  "text/html",
		CacheControl: "public, max-age=60",
		Crc32c:       crcSum(buf.Bytes()),
		Size:         uint64(buf.Len()), // used by crcEq but not API
	}

	if old, ok := d.Objects["index.html"]; ok && crcEq(old, obj) {
		return nil // up to date!
	}

	writeReq := service.Objects.Insert(d.Bucket, obj)
	writeReq.Media(&buf)

	fmt.Printf("Writing gs://%s/%s\n", d.Bucket, obj.Name)
	_, err = writeReq.Do()
	return err
}
