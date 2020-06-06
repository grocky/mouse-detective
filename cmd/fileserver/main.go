package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
)

func main() {
	filepath := flag.String("path", ".", "the directory to serve")
	flag.Parse()
	port := 9090
	log.Printf("serving %s on port %d\n", *filepath, port)
	fs := http.FileServer(http.Dir(*filepath))
	// if accessing from a container to the host, then use the following URL template
	// http://host.docker.internal:9090/${pathToFile}
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), logRequestHandler(fs)))
}

func logRequestHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		h.ServeHTTP(w, r)
	})
}
