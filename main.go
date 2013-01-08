package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

var templates *template.Template

var staticPath = flag.String("static", "static", "Path to the static content")

func init() {
	var err error
	templates, err = template.ParseGlob(fmt.Sprintf("html/%c.html", '*'))
	if err != nil {
		panic("Couldn't parse templates.")
	}
	http.HandleFunc("/", serveHome)
	http.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html",
		map[string]interface{}{})
}

func main() {
	addr := flag.String("addr", ":8066", "http listen address")

	flag.Parse()

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
