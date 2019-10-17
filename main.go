package main

import (
	"context"
	"encoding/json"
	// "html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonwho/bootleg-fs/lrucache"
)

// 1 MB
const maxMemory = 1 * 1024 * 1024

var cache *lrucache.LRUCache

func main() {
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	cache = lrucache.New(3)

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/ping", handlePing)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/download", handleDownload)

	// start server without blocking
	go func() {
		log.Fatal(srv.ListenAndServe())
	}()

	log.Println("Server running on port 8080. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	// Block until signal received
	<-sc

	// Create a deadline to wait for
	gracefulTimeout := time.Second * 7
	ctx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()

	// Doesn't block if srv has no connections
	srv.Shutdown(ctx)
	log.Println("Shutting down server...")
	os.Exit(0)
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong!\n"))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "application/json")

		// this totally doesn't work lol
		if err := r.ParseMultipartForm(maxMemory); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{ "error": "file size too large" }`))
			return
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			log.Println("Error retrieving file from form-data")
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest) // find better status
			w.Write([]byte(`{ "error": "file lost" }`))
			return
		}
		defer file.Close()
		log.Printf("Uploaded File: %+v\n", handler.Filename)
		log.Printf("Uploaded File Size: %+v\n", handler.Size)
		log.Printf("MIME Header: %+v\n", handler.Header)

		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // find better status
			w.Write([]byte(`{ "error": "file lost" }`))
			return
		}

		// cache it
		cache.Set(handler.Filename, fileBytes)

		jsonResponse, err := json.Marshal(struct {
			Data []byte `json:"data"`
		}{Data: fileBytes})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // find better status
			w.Write([]byte(`{ "error": "json failure" }`))
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write(jsonResponse)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("request not found"))
		return
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")

		keys, ok := r.URL.Query()["key"]
		if !ok || len(keys) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{ "error": "key is missing" }`))
			return
		}
		key := keys[0]
		data := cache.Get(key)
		if data == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{ "error": "no file found for key" }`))
			return
		}

		jsonResponse, err := json.Marshal(struct {
			Data []byte `json:"data"`
		}{Data: data})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // find better status
			w.Write([]byte(`{ "error": "json failure" }`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("request not found"))
	}
}
