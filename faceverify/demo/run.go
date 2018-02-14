package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	var (
		addr = flag.String("addr", ":9000", "listen address")
	)
	flag.Parse()
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Println("listening on", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalln(err)
	}
}
