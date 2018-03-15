package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/machinebox/sdk-go/boxutil"
	"github.com/machinebox/sdk-go/facebox"
	pb "gopkg.in/cheggaaa/pb.v1"
)

//Main looks so small and lonely
func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	faceboxClient := facebox.New("http://localhost:8080")
	log.Println("waiting for box to be ready...")
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := boxutil.WaitForReady(ctx, faceboxClient)
	if err != nil {
		if err == context.Canceled {
			log.Fatalln("timed out waiting for box to be ready")
		}
		log.Fatalln(err)
	}
	log.Println("box ready")
	r, err := os.Open("namesandpaths.txt")
	if err != nil {
		return err
	}
	defer r.Close()
	type Item struct {
		Filename, Celebname string
	}
	itemsChan := make(chan Item)
	defer close(itemsChan)
	for i := 0; i < 4; i++ {
		go func() {
			for item := range itemsChan {
				err := teachFromFile(faceboxClient, item.Filename, item.Celebname)
				if err != nil {
					//log.Println("ERROR: teachFromFile:", err)
				}
			}
		}()
	}

	bar := pb.StartNew(460723)

	s := bufio.NewScanner(r)
	for s.Scan() {
		//log.Println(s.Text())
		subs := strings.Split(s.Text(), ",")
		celebname := subs[0]
		filename := "imdb_crop/" + subs[1]
		itemsChan <- Item{
			Filename:  filename,
			Celebname: celebname,
		}
		bar.Increment()
	}
	bar.Finish()
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}

func teachFromFile(faceboxClient *facebox.Client, filename, name string) error {
	//log.Printf("Now teaching %v (%v)\n", filename, name)
	r, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer r.Close()
	err = faceboxClient.Teach(r, filename, name)
	if err != nil {
		return err
	}
	return nil
}
