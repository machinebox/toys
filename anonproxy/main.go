package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/machinebox/sdk-go/facebox"
)

func main() {
	var (
		addr        = flag.String("addr", "localhost:8000", "Listen address")
		faceboxAddr = flag.String("facebox", "http://localhost:8080", "Facebox address")
	)
	flag.Parse()
	client := &http.Client{Timeout: 10 * time.Second}
	fb := facebox.New(*faceboxAddr)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		urlStr := r.URL.Query().Get("src")
		log.Println(urlStr)
		u, err := url.Parse(urlStr)
		if err != nil {
			http.Error(w, "src: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !u.IsAbs() {
			http.Error(w, "src: absolute url required", http.StatusBadRequest)
			return
		}
		resp, err := client.Get(urlStr)
		if err != nil {
			http.Error(w, "download failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			http.Error(w, "download failed: "+resp.Status, resp.StatusCode)
			return
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		img, format, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			http.Error(w, "image: "+err.Error(), http.StatusInternalServerError)
			return
		}
		faces, err := fb.Check(bytes.NewReader(b))
		if err != nil {
			http.Error(w, "facebox: "+err.Error(), http.StatusInternalServerError)
			return
		}
		anonImg := anonymise(img, faces)
		switch format {
		case "jpeg":
			w.Header().Set("Content-Type", "image/jpg")
			if err := jpeg.Encode(w, anonImg, &jpeg.Options{Quality: 100}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "gif":
			w.Header().Set("Content-Type", "image/gif")
			if err := gif.Encode(w, anonImg, nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "png":
			w.Header().Set("Content-Type", "image/png")
			if err := png.Encode(w, anonImg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "unsupported format: "+format, http.StatusInternalServerError)
			return
		}
	})
	fmt.Println("Facebox at", *faceboxAddr)
	fmt.Println("listening on", *addr)
	fmt.Println("usage:", "http://"+*addr+"/?src=http://...")
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalln(err)
	}
}

// anonymise produces a new image with faces redacted.
// see https://becominghuman.ai/anonymising-images-with-go-and-machine-box-fd0866adb9f5
func anonymise(src image.Image, faces []facebox.Face) image.Image {
	dstImage := image.NewRGBA(src.Bounds())
	draw.Draw(dstImage, src.Bounds(), src, image.ZP, draw.Src)
	for _, face := range faces {
		faceRect := image.Rect(
			face.Rect.Left,
			face.Rect.Top,
			face.Rect.Left+face.Rect.Width,
			face.Rect.Top+face.Rect.Height,
		)
		facePos := image.Pt(face.Rect.Left, face.Rect.Top)
		draw.Draw(
			dstImage,
			faceRect,
			&image.Uniform{color.Black},
			facePos,
			draw.Src)
	}
	return dstImage
}
