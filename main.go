package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anacrolix/tagflag"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	var flags struct {
		RootTemplateFile string
		SourceDir        string
		DestDir          string
		tagflag.StartPos
		Command string
		tagflag.ExcessArgs
	}
	tagflag.Parse(&flags)
	err := func() error {
		switch flags.Command {
		case "build":
			tagflag.ParseArgs(nil, flags.ExcessArgs)
			return build(flags.RootTemplateFile, flags.SourceDir, flags.DestDir)
		case "serve":
			var serveFlags struct {
				Addr string
			}
			tagflag.ParseArgs(&serveFlags, flags.ExcessArgs)
			return serve(flags.RootTemplateFile, flags.SourceDir, serveFlags.Addr)
		default:
			return fmt.Errorf("bad command %q", flags.Command)
		}
	}()
	if err != nil {
		log.Fatal(err)
	}
}

func build(rootTemplateFile, sourceDir, destDir string) error {
	log.Printf("parsing %q", rootTemplateFile)
	t := template.Must(template.ParseFiles(rootTemplateFile))
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		log.Print("parsing %q", path)
		t1 := template.Must(template.Must(t.Clone()).ParseFiles(path))
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			panic(err)
		}
		destPath := filepath.Join(destDir, relPath)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
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
	t.Execute(w, nil)
}

func serveStatic(w http.ResponseWriter, r *http.Request) bool {
	p := filepath.Join("static", filepath.FromSlash(r.URL.Path))
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

func serve(rootTemplateFile, sourceDir, addr string) error {
	return http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveStatic(w, r) {
			return
		}
		root := template.Must(template.ParseFiles(rootTemplateFile))
		p := filepath.Join(sourceDir, filepath.FromSlash(r.URL.Path))
		servePath(root, sourceDir, p, w)
	}))
}
