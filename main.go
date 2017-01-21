package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anacrolix/missinggo/x"
	"github.com/anacrolix/tagflag"
)

const (
	rootTemplateFile = "base.html"
	sourceDir        = "source"
	destDir          = "docs"
	staticDir        = "static"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	var flags struct {
		tagflag.StartPos
		Command string
		tagflag.ExcessArgs
	}
	tagflag.Parse(&flags)
	err := func() error {
		switch flags.Command {
		case "build":
			tagflag.ParseArgs(nil, flags.ExcessArgs)
			return build()
		case "serve":
			var serveFlags struct {
				Addr string
			}
			tagflag.ParseArgs(&serveFlags, flags.ExcessArgs)
			return serve(serveFlags.Addr)
		default:
			return fmt.Errorf("bad command %q", flags.Command)
		}
	}()
	if err != nil {
		log.Fatal(err)
	}
}

func copy(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(srcDir, path)
		x.Pie(err)
		dp := filepath.Join(destDir, relPath)
		os.MkdirAll(filepath.Dir(dp), 0755)
		log.Printf("copying %q", dp)
		df, err := os.OpenFile(dp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		x.Pie(err)
		defer df.Close()
		sf, err := os.Open(path)
		x.Pie(err)
		defer sf.Close()
		_, err = io.Copy(df, sf)
		x.Pie(err)
		return nil
	})
}

func build() error {
	if err := copy(staticDir, destDir); err != nil {
		return err
	}
	log.Printf("parsing %q", rootTemplateFile)
	t := template.Must(template.ParseFiles(rootTemplateFile))
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		log.Printf("parsing %q", path)
		t1 := template.Must(template.Must(t.Clone()).ParseFiles(path))
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			panic(err)
		}
		destPath := filepath.Join(destDir, relPath)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatalf("not opening destination file: %s", err)
		}
		defer f.Close()
		log.Printf("generating %q", destPath)
		err = t1.Execute(f, nil)
		if err != nil {
			panic(err)
		}
		return nil
	})
}

func servePath(root *template.Template, sourceDir, filePath string, w http.ResponseWriter) {
	fi, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		http.Error(w, "not exist", http.StatusNotFound)
		return
	}
	if err != nil {
		panic(err)
	}
	if fi.IsDir() {
		servePath(root, sourceDir, filepath.Join(filePath, "index.html"), w)
		return
	}
	t := template.Must(template.Must(root.Clone()).ParseFiles(filePath))
	log.Printf("executing template file %q", filePath)
	err = t.Execute(w, nil)
	if err != nil {
		panic(err)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) bool {
	p := filepath.Join(staticDir, filepath.FromSlash(r.URL.Path))
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		panic(err)
	}
	if info.IsDir() {
		return false
	}
	log.Printf("serving %q", p)
	http.ServeFile(w, r, p)
	return true
}

func serve(addr string) error {
	return http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveStatic(w, r) {
			return
		}
		root := template.Must(template.ParseFiles(rootTemplateFile))
		p := filepath.Join(sourceDir, filepath.FromSlash(r.URL.Path))
		servePath(root, sourceDir, p, w)
	}))
}
