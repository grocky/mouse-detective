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
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), fs))
}
