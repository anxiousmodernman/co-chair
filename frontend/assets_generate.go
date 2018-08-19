//+build generate

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var fs http.FileSystem = http.Dir("static")

	err := vfsgen.Generate(fs, vfsgen.Options{
		Filename:     "bundle/bundle.go",
		PackageName:  "bundle",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatalln(err)
	}
	err = generateAbsolutePaths("static")
	if err != nil {
		panic(err)
	}

}

func generateAbsolutePaths(dir string) error {

	type entries struct {
		Entries []string
	}
	var paths entries
	// collect paths
	visit := func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			trimmed := strings.TrimPrefix(path, "static/")
			fmt.Println("adding path", trimmed)
			paths.Entries = append(paths.Entries, trimmed)
		}
		return nil
	}
	err := filepath.Walk(dir, visit)
	if err != nil {
		return err
	}

	var t = template.Must(template.New("entries").Funcs(template.FuncMap{
		"quote": strconv.Quote,
	}).Parse(`package bundle

var Entries = []string{
		{{range .Entries}}
	"{{ .}}",
		{{end}}
	}
	`))

	f, err := os.Create("bundle/entries.go")
	if err != nil {
		return err
	}
	defer f.Close()

	err = t.ExecuteTemplate(f, "entries", paths)
	if err != nil {
		return err
	}
	return nil
}
