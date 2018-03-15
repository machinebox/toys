package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var (
		addr = flag.String("addr", ":9000", "listen address")
	)
	flag.Parse()
	// fbcmd := exec.Command("/app/facebox")
	// fbcmd.Stdout = os.Stdout
	// if err := fbcmd.Start(); err != nil {
	// 	return errors.Wrap(err, "start facebox")
	// }
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Println("listening on", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		return err
	}
	return nil
}
